use anyhow::{Context, Result, bail};
use byteorder::{ByteOrder, LittleEndian};
use glam::{DQuat, DVec3};
use las::{Point, Writer};
use rayon::prelude::*;
use std::collections::HashSet;
use std::fs::File;
use std::io::{BufRead, BufReader, BufWriter, Read, Seek, SeekFrom};
use std::path::Path;

use crate::transform::{self, TransformConfig};

pub(crate) struct PoseSample {
    pub timestamp_text: String,
    pub translation: DVec3,
    pub rotation: DQuat,
}

pub(crate) struct PcdFile {
    path: std::path::PathBuf,
    schema: PcdSchema,
    point_count: usize,
    data_offset: u64,
    data_encoding: String,
}

#[derive(Clone, Debug)]
struct PcdSchema {
    point_stride: usize,
    x_field: PcdField,
    y_field: PcdField,
    z_field: PcdField,
    intensity_field: Option<PcdField>,
}

#[derive(Clone, Copy, Debug)]
struct PcdField {
    kind: PcdFieldKind,
    size: usize,
    offset: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum PcdFieldKind {
    Signed,
    Unsigned,
    Float,
}

// ---- PCD header parsing ----

impl PcdFile {
    pub fn open(path: &Path) -> Result<Self> {
        let mut file =
            File::open(path).with_context(|| format!("无法打开 PCD: {}", path.display()))?;
        let mut reader = BufReader::new(&mut file);
        let mut line = String::new();
        let mut field_names = Vec::<String>::new();
        let mut sizes = Vec::<usize>::new();
        let mut kinds = Vec::<PcdFieldKind>::new();
        let mut counts = Vec::<usize>::new();
        let mut point_count = None;
        let mut data_offset = 0u64;

        let data_encoding = loop {
            line.clear();
            let bytes = reader
                .read_line(&mut line)
                .with_context(|| format!("读取 PCD 头失败: {}", path.display()))?;
            if bytes == 0 {
                bail!("读取 PCD 头时遇到 EOF: {}", path.display());
            }
            data_offset += bytes as u64;
            let trimmed = line.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                continue;
            }
            let mut parts = trimmed.split_whitespace();
            let key = parts.next().unwrap_or_default().to_ascii_uppercase();
            let values: Vec<_> = parts.collect();
            match key.as_str() {
                "FIELDS" => {
                    field_names = values.iter().map(|v| v.to_ascii_lowercase()).collect();
                }
                "SIZE" => {
                    sizes = values
                        .iter()
                        .map(|v| v.parse::<usize>())
                        .collect::<std::result::Result<_, _>>()
                        .with_context(|| format!("PCD SIZE 字段非法: {}", path.display()))?;
                }
                "TYPE" => {
                    kinds = values
                        .iter()
                        .map(|v| parse_field_kind(v, path))
                        .collect::<Result<_>>()?;
                }
                "COUNT" => {
                    counts = values
                        .iter()
                        .map(|v| v.parse::<usize>())
                        .collect::<std::result::Result<_, _>>()
                        .with_context(|| format!("PCD COUNT 字段非法: {}", path.display()))?;
                }
                "POINTS" => {
                    point_count = Some(
                        values
                            .first()
                            .context("PCD 缺少 POINTS 值")?
                            .parse::<usize>()
                            .with_context(|| format!("PCD POINTS 字段非法: {}", path.display()))?,
                    );
                }
                "DATA" => {
                    break values
                        .first()
                        .context("PCD 缺少 DATA 值")?
                        .to_ascii_lowercase();
                }
                _ => {}
            }
        };

        let point_count = point_count.context("PCD 缺少 POINTS 头字段")?;
        if counts.is_empty() {
            counts = vec![1; field_names.len()];
        }
        if field_names.len() != sizes.len()
            || field_names.len() != kinds.len()
            || field_names.len() != counts.len()
        {
            bail!("PCD 头字段数量不匹配: {}", path.display());
        }

        let mut point_stride = 0usize;
        let mut x_field = None;
        let mut y_field = None;
        let mut z_field = None;
        let mut intensity_field = None;
        for (((name, size), kind), count) in field_names
            .iter()
            .zip(sizes.iter())
            .zip(kinds.iter())
            .zip(counts.iter())
        {
            if *count != 1 {
                point_stride = point_stride
                    .checked_add(size.saturating_mul(*count))
                    .context("PCD 点步长溢出")?;
                continue;
            }
            let field = PcdField {
                kind: *kind,
                size: *size,
                offset: point_stride,
            };
            match name.as_str() {
                "x" => x_field = Some(field),
                "y" => y_field = Some(field),
                "z" => z_field = Some(field),
                "intensity" => intensity_field = Some(field),
                _ => {}
            }
            point_stride = point_stride
                .checked_add(size.saturating_mul(*count))
                .context("PCD 点步长溢出")?;
        }

        Ok(Self {
            path: path.to_path_buf(),
            schema: PcdSchema {
                point_stride,
                x_field: x_field.context("PCD 缺少 x 字段")?,
                y_field: y_field.context("PCD 缺少 y 字段")?,
                z_field: z_field.context("PCD 缺少 z 字段")?,
                intensity_field,
            },
            point_count,
            data_offset,
            data_encoding,
        })
    }

    /// 加载 PCD 点云，可选 ENU 位姿 和/或 origin 偏转。
    /// pose: 将 LiDAR 局部坐标转为 ENU 世界坐标
    /// transform: 将 ENU 世界坐标偏转到 UTM
    pub fn load_points(
        &self,
        pose: Option<&PoseSample>,
        transform: Option<&TransformConfig>,
    ) -> Result<Vec<Point>> {
        if self.data_encoding != "binary" {
            bail!("仅支持 binary 编码的 PCD，当前为: {}", self.data_encoding);
        }
        if self.point_count == 0 {
            return Ok(Vec::new());
        }

        let mut file = File::open(&self.path)
            .with_context(|| format!("无法打开 PCD: {}", self.path.display()))?;
        file.seek(SeekFrom::Start(self.data_offset))
            .with_context(|| format!("无法定位到 PCD 数据区: {}", self.path.display()))?;

        let payload_len = self
            .point_count
            .checked_mul(self.schema.point_stride)
            .context("PCD payload 大小溢出")?;
        let mut payload = vec![0u8; payload_len];
        file.read_exact(&mut payload)
            .with_context(|| format!("读取点记录失败（期望 {} 字节）: {}", payload_len, self.path.display()))?;

        let stride = self.schema.point_stride;
        let x_field = self.schema.x_field;
        let y_field = self.schema.y_field;
        let z_field = self.schema.z_field;
        let intensity_field = self.schema.intensity_field;

        // 帧内并行解析点云
        let mut points: Vec<Point> = payload
            .par_chunks(stride)
            .take(self.point_count)
            .map(|record| {
                let local = DVec3::new(
                    x_field.scalar_as_f64(record).unwrap_or(0.0),
                    y_field.scalar_as_f64(record).unwrap_or(0.0),
                    z_field.scalar_as_f64(record).unwrap_or(0.0),
                );

                // 步骤1: ENU 位姿变换
                let world = if let Some(pose) = pose {
                    apply_pose(local, pose)
                } else {
                    local
                };

                let intensity = intensity_field
                    .and_then(|f| f.scalar_as_f64(record))
                    .map(quantize_intensity)
                    .unwrap_or(0);

                let mut point = Point::default();
                point.x = world.x;
                point.y = world.y;
                point.z = world.z;
                point.intensity = intensity;
                point
            })
            .collect();

        // 立即释放原始 payload 内存，避免与 points 同时占用
        drop(payload);

        // 步骤2: origin 偏转（并行）
        if let Some(config) = transform {
            points.par_iter_mut().for_each(|point| {
                // config 已在 build_config 中校验，此处不会失败
                transform::apply_transform(point, config)
                    .expect("坐标变换失败");
            });
        }

        Ok(points)
    }
}

impl PcdField {
    fn scalar_as_f64(&self, record: &[u8]) -> Option<f64> {
        let bytes = record.get(self.offset..self.offset + self.size)?;
        match (self.kind, self.size) {
            (PcdFieldKind::Unsigned, 1) => Some(f64::from(bytes[0])),
            (PcdFieldKind::Unsigned, 2) => Some(f64::from(LittleEndian::read_u16(bytes))),
            (PcdFieldKind::Unsigned, 4) => Some(f64::from(LittleEndian::read_u32(bytes))),
            (PcdFieldKind::Unsigned, 8) => Some(LittleEndian::read_u64(bytes) as f64),
            (PcdFieldKind::Signed, 1) => Some(f64::from(i8::from_le_bytes([bytes[0]]))),
            (PcdFieldKind::Signed, 2) => Some(f64::from(LittleEndian::read_i16(bytes))),
            (PcdFieldKind::Signed, 4) => Some(f64::from(LittleEndian::read_i32(bytes))),
            (PcdFieldKind::Signed, 8) => Some(LittleEndian::read_i64(bytes) as f64),
            (PcdFieldKind::Float, 4) => Some(f64::from(LittleEndian::read_f32(bytes))),
            (PcdFieldKind::Float, 8) => Some(LittleEndian::read_f64(bytes)),
            _ => None,
        }
    }
}

fn parse_field_kind(value: &str, path: &Path) -> Result<PcdFieldKind> {
    match value {
        "I" => Ok(PcdFieldKind::Signed),
        "U" => Ok(PcdFieldKind::Unsigned),
        "F" => Ok(PcdFieldKind::Float),
        _ => bail!("PCD TYPE {:?} 不支持: {}", value, path.display()),
    }
}

fn apply_pose(local: DVec3, pose: &PoseSample) -> DVec3 {
    pose.translation + pose.rotation * local
}

fn quantize_intensity(value: f64) -> u16 {
    if !value.is_finite() || value <= 0.0 {
        0
    } else {
        value.round().clamp(0.0, 65535.0) as u16
    }
}

// ---- ENU 位姿文件加载 ----

pub(crate) fn load_enu_poses(path: &Path) -> Result<Vec<PoseSample>> {
    let file =
        File::open(path).with_context(|| format!("无法打开 ENU 文件: {}", path.display()))?;
    let reader = BufReader::new(file);
    let mut poses = Vec::new();
    let mut seen_timestamps = HashSet::new();

    for (line_no, line) in reader.lines().enumerate() {
        let line = line.with_context(|| format!("读取 {}:{} 失败", path.display(), line_no + 1))?;
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }
        let fields: Vec<_> = trimmed.split_whitespace().collect();
        if fields.len() != 8 {
            bail!(
                "ENU 第 {} 行列数不正确，期望 8 列，实际 {}",
                line_no + 1,
                fields.len()
            );
        }
        if !seen_timestamps.insert(fields[0].to_string()) {
            bail!("ENU 中存在重复时间戳: {}", fields[0]);
        }

        let tx = parse_enu_f64(fields[1], path, line_no + 1, "tx")?;
        let ty = parse_enu_f64(fields[2], path, line_no + 1, "ty")?;
        let tz = parse_enu_f64(fields[3], path, line_no + 1, "tz")?;
        let qx = parse_enu_f64(fields[4], path, line_no + 1, "qx")?;
        let qy = parse_enu_f64(fields[5], path, line_no + 1, "qy")?;
        let qz = parse_enu_f64(fields[6], path, line_no + 1, "qz")?;
        let qw = parse_enu_f64(fields[7], path, line_no + 1, "qw")?;
        let quat = DQuat::from_xyzw(qx, qy, qz, qw);
        if !quat.is_finite() || quat.length_squared() == 0.0 {
            bail!("ENU 第 {} 行四元数无效", line_no + 1);
        }
        poses.push(PoseSample {
            timestamp_text: fields[0].to_string(),
            translation: DVec3::new(tx, ty, tz),
            rotation: quat.normalize(),
        });
    }
    Ok(poses)
}

fn parse_enu_f64(value: &str, path: &Path, line_no: usize, field_name: &str) -> Result<f64> {
    value.parse::<f64>().with_context(|| {
        format!(
            "解析 {} 的第 {} 行字段 {} 失败，原始值 {:?}",
            path.display(),
            line_no,
            field_name,
            value
        )
    })
}

// ---- LAZ 写出 ----

pub(crate) fn write_laz(
    path: &Path,
    points: &[Point],
    epsg: Option<u16>,
) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("无法创建目录: {}", parent.display()))?;
    }

    let header = transform::build_point_cloud_header(points, epsg)?;

    let file =
        File::create(path).with_context(|| format!("无法创建输出文件: {}", path.display()))?;
    let buffer = BufWriter::with_capacity(16 * 1024 * 1024, file);
    let mut writer = Writer::new(buffer, header)
        .with_context(|| format!("无法创建 LAS writer: {}", path.display()))?;

    const CHUNK: usize = 250_000;
    for chunk in points.chunks(CHUNK) {
        writer
            .write_points(chunk)
            .with_context(|| format!("写出点记录失败: {}", path.display()))?;
    }
    Ok(())
}
