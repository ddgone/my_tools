use std::collections::HashMap;
use std::fs::{self, File};
use std::io::{BufRead, BufReader, Read, Seek, SeekFrom};
use std::path::{Path, PathBuf};

use anyhow::{Context, Result, bail};
use byteorder::{ByteOrder, LittleEndian};
use glam::DMat3;
use las::Point;

#[derive(Clone, Debug)]
pub struct PcdSchema {
    pub point_stride: usize,
    pub x_field: PcdField,
    pub y_field: PcdField,
    pub z_field: PcdField,
    pub intensity_field: Option<PcdField>,
}

#[derive(Clone, Copy, Debug)]
pub struct PcdField {
    pub kind: PcdFieldKind,
    pub size: usize,
    pub offset: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PcdFieldKind {
    Signed,
    Unsigned,
    Float,
}

#[derive(Debug)]
struct ParsedPcdHeader {
    schema: PcdSchema,
    points: usize,
    data_offset: u64,
    data_encoding: String,
}

#[derive(Debug, Clone)]
pub struct PcdFrame {
    pub timestamp_ms: u64,
    pub path: PathBuf,
    pub point_count: usize,
    pub data_offset: u64,
    pub schema: PcdSchema,
}

/// 扫描 PCD 目录，返回 毫秒时间戳 -> PcdFrame 的映射
pub fn scan_pcd_frames(pcd_dir: &Path) -> Result<HashMap<u64, PcdFrame>> {
    let mut frames = HashMap::new();
    let mut entries: Vec<_> = fs::read_dir(pcd_dir)
        .with_context(|| format!("无法读取 PCD 目录: {}", pcd_dir.display()))?
        .collect::<std::result::Result<Vec<_>, _>>()
        .with_context(|| format!("枚举 PCD 目录失败: {}", pcd_dir.display()))?;
    entries.sort_by_key(|entry| entry.path());

    for entry in entries {
        if !entry.file_type()?.is_file() {
            continue;
        }
        let file_path = entry.path();
        if file_path.extension().and_then(|ext| ext.to_str()) != Some("pcd") {
            continue;
        }
        let stem = match file_path.file_stem().and_then(|stem| stem.to_str()) {
            Some(s) => s,
            None => continue,
        };
        // 解析 PCD 文件名时间戳: seconds.microseconds000
        let timestamp_s: f64 = match stem.parse() {
            Ok(v) => v,
            Err(_) => continue,
        };
        let timestamp_ms = (timestamp_s * 1000.0).round() as u64;

        let metadata = entry.metadata()?;
        if metadata.len() == 0 {
            continue;
        }
        let header = match parse_pcd_header_file(&file_path) {
            Ok(h) => h,
            Err(_) => continue,
        };
        if header.data_encoding != "binary" || header.points == 0 {
            continue;
        }

        frames.insert(
            timestamp_ms,
            PcdFrame {
                timestamp_ms,
                path: file_path,
                point_count: header.points,
                data_offset: header.data_offset,
                schema: header.schema,
            },
        );
    }
    Ok(frames)
}

/// 加载单个 PCD 帧的全部点数据（局部坐标），不应用位姿变换
pub fn load_frame_raw(frame: &PcdFrame) -> Result<Vec<Point>> {
    let mut file =
        File::open(&frame.path).with_context(|| format!("无法打开 PCD: {}", frame.path.display()))?;
    file.seek(SeekFrom::Start(frame.data_offset))
        .with_context(|| format!("无法定位到 PCD 数据区: {}", frame.path.display()))?;
    let payload_len = frame
        .point_count
        .checked_mul(frame.schema.point_stride)
        .context("PCD payload 大小溢出")?;
    let mut payload = vec![0u8; payload_len];
    file.read_exact(&mut payload)
        .with_context(|| format!("读取点记录失败: {}", frame.path.display()))?;

    let mut points = Vec::with_capacity(frame.point_count);
    for point_idx in 0..frame.point_count {
        let base = point_idx * frame.schema.point_stride;
        let record = &payload[base..base + frame.schema.point_stride];
        let mut point = Point::default();
        point.x = frame.schema.x_field.scalar_as_f64(record).unwrap_or(0.0);
        point.y = frame.schema.y_field.scalar_as_f64(record).unwrap_or(0.0);
        point.z = frame.schema.z_field.scalar_as_f64(record).unwrap_or(0.0);
        point.intensity = frame
            .schema
            .intensity_field
            .and_then(|field| field.scalar_as_f64(record))
            .map(quantize_intensity)
            .unwrap_or(0);
        points.push(point);
    }
    Ok(points)
}

/// 构建方位旋转矩阵：车辆坐标系(X=前,Y=右,Z=上) → ENU(X=东,Y=北,Z=上)
///
/// azimuth: 顺时针从北 (0°=N, 90°=E, 270°=W)
/// 不做 pitch/roll 偏转（值很小，且会引入Z轴层间错位）
///
/// M_az = [sin az,  cos az, 0; cos az, -sin az, 0; 0, 0, 1]
pub fn build_rotation(azimuth_deg: f64) -> DMat3 {
    let az = azimuth_deg.to_radians();
    let sa = az.sin();
    let ca = az.cos();
    DMat3::from_cols(
        glam::DVec3::new(sa, ca, 0.0),
        glam::DVec3::new(ca, -sa, 0.0),
        glam::DVec3::new(0.0, 0.0, 1.0),
    )
}

fn parse_pcd_header_file(path: &Path) -> Result<ParsedPcdHeader> {
    let mut file = File::open(path).with_context(|| format!("无法打开 PCD: {}", path.display()))?;
    parse_pcd_header(&mut file, path)
}

fn parse_pcd_header(file: &mut File, path: &Path) -> Result<ParsedPcdHeader> {
    let mut reader = BufReader::new(file);
    let mut line = String::new();
    let mut field_names = Vec::<String>::new();
    let mut sizes = Vec::<usize>::new();
    let mut kinds = Vec::<PcdFieldKind>::new();
    let mut counts = Vec::<usize>::new();
    let mut points = None;
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
                field_names = values
                    .iter()
                    .map(|value| value.to_ascii_lowercase())
                    .collect()
            }
            "SIZE" => {
                sizes = values
                    .iter()
                    .map(|value| value.parse::<usize>())
                    .collect::<std::result::Result<_, _>>()
                    .context("PCD SIZE 字段非法")?;
            }
            "TYPE" => {
                kinds = values
                    .iter()
                    .map(|value| parse_field_kind(value))
                    .collect::<Result<_>>()?;
            }
            "COUNT" => {
                counts = values
                    .iter()
                    .map(|value| value.parse::<usize>())
                    .collect::<std::result::Result<_, _>>()
                    .context("PCD COUNT 字段非法")?;
            }
            "POINTS" => {
                points = Some(
                    values
                        .first()
                        .context("PCD 缺少 POINTS 值")?
                        .parse::<usize>()
                        .context("PCD POINTS 字段非法")?,
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

    let points = points.context("PCD 缺少 POINTS 头字段")?;
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

    Ok(ParsedPcdHeader {
        schema: PcdSchema {
            point_stride,
            x_field: x_field.context("PCD 缺少 x 字段")?,
            y_field: y_field.context("PCD 缺少 y 字段")?,
            z_field: z_field.context("PCD 缺少 z 字段")?,
            intensity_field,
        },
        points,
        data_offset,
        data_encoding,
    })
}

fn parse_field_kind(value: &str) -> Result<PcdFieldKind> {
    match value {
        "I" => Ok(PcdFieldKind::Signed),
        "U" => Ok(PcdFieldKind::Unsigned),
        "F" => Ok(PcdFieldKind::Float),
        _ => bail!("PCD TYPE {value:?} 不支持"),
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

fn quantize_intensity(value: f64) -> u16 {
    if !value.is_finite() || value <= 0.0 {
        0
    } else {
        value.round().clamp(0.0, 65535.0) as u16
    }
}
