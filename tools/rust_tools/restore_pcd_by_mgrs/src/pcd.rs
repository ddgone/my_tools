//! PCD（binary）读取与 LAS 写出。
//!
//! LAS 头与原 Python 工具保持一致：版本 1.4、点格式 3（RGB+GPS 时间）、
//! 比例 0.01、偏移取 MGRS 1km 块西南角，CRS 以 GeoTIFF GeoKey VLR 写入。

use anyhow::{Context, Result, bail};
use byteorder::{ByteOrder, LittleEndian};
use las::{Builder, Color, Point, Vlr, Writer};
use rayon::prelude::*;
use std::fs::File;
use std::io::{BufRead, BufReader, BufWriter, Read, Seek, SeekFrom};
use std::path::Path;

use crate::mgrs::MgrsUtmOffset;

/// 一个点：局部坐标 + 可选 RGB（u16 通道，已按 8bit→16bit 拉伸）。
pub(crate) struct CloudPoint {
    pub x: f64,
    pub y: f64,
    pub z: f64,
    pub color: Option<(u16, u16, u16)>,
}

struct PcdSchema {
    point_stride: usize,
    x_field: PcdField,
    y_field: PcdField,
    z_field: PcdField,
    rgb_field: Option<PcdField>,
}

#[derive(Clone, Copy)]
struct PcdField {
    kind: PcdFieldKind,
    size: usize,
    offset: usize,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum PcdFieldKind {
    Signed,
    Unsigned,
    Float,
}

struct PcdFile {
    path: std::path::PathBuf,
    schema: PcdSchema,
    point_count: usize,
    data_offset: u64,
    data_encoding: String,
}

impl PcdFile {
    fn open(path: &Path) -> Result<Self> {
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
                        .map(|v| parse_field_kind(v))
                        .collect::<Option<_>>()
                        .context("PCD TYPE 字段非法")?;
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
        let mut rgb_field = None;
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
                "rgb" | "rgba" if *size == 4 => rgb_field = Some(field),
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
                rgb_field,
            },
            point_count,
            data_offset,
            data_encoding,
        })
    }

    fn load_points(&self) -> Result<Vec<CloudPoint>> {
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
        file.read_exact(&mut payload).with_context(|| {
            format!(
                "读取点记录失败（期望 {} 字节）: {}",
                payload_len,
                self.path.display()
            )
        })?;

        let stride = self.schema.point_stride;
        let x_field = self.schema.x_field;
        let y_field = self.schema.y_field;
        let z_field = self.schema.z_field;
        let rgb_field = self.schema.rgb_field;

        let points: Vec<CloudPoint> = payload
            .par_chunks(stride)
            .take(self.point_count)
            .map(|record| CloudPoint {
                x: x_field.scalar_as_f64(record).unwrap_or(0.0),
                y: y_field.scalar_as_f64(record).unwrap_or(0.0),
                z: z_field.scalar_as_f64(record).unwrap_or(0.0),
                color: rgb_field.and_then(|f| packed_rgb(record, f)),
            })
            .collect();
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

fn parse_field_kind(value: &str) -> Option<PcdFieldKind> {
    match value {
        "I" => Some(PcdFieldKind::Signed),
        "U" => Some(PcdFieldKind::Unsigned),
        "F" => Some(PcdFieldKind::Float),
        _ => None,
    }
}

/// PCD 的 rgb/rgba 字段按位打包 RGB888（以 f32 或 u32 存储），解出并拉伸到 u16。
/// 拉伸系数 257 与 open3d 归一化后 *65535 的行为一致。
fn packed_rgb(record: &[u8], field: PcdField) -> Option<(u16, u16, u16)> {
    let bytes = record.get(field.offset..field.offset + 4)?;
    let bits = LittleEndian::read_u32(bytes);
    let scale = |channel: u32| (channel.clamp(0, 255) * 257) as u16;
    Some((
        scale((bits >> 16) & 0xFF),
        scale((bits >> 8) & 0xFF),
        scale(bits & 0xFF),
    ))
}

/// 读取 PCD 点云（局部坐标 + 可选颜色）。
pub(crate) fn read_pcd_points(path: &Path) -> Result<Vec<CloudPoint>> {
    PcdFile::open(path)?.load_points()
}

/// 写出 LAS：点坐标为局部坐标加 UTM 偏移后的绝对值。
pub(crate) fn write_las(path: &Path, points: &[CloudPoint], offset: &MgrsUtmOffset) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("无法创建目录: {}", parent.display()))?;
    }

    let mut builder = Builder::from((1, 4));
    builder.generating_software = "restore_pcd_by_mgrs".to_string();
    builder.system_identifier = "pcd_convert".to_string();
    builder.point_format = las::point::Format::new(3).context("创建 LAS 点格式失败")?;
    builder.transforms.x.scale = 0.01;
    builder.transforms.y.scale = 0.01;
    builder.transforms.z.scale = 0.01;
    builder.transforms.x.offset = offset.offset_x as f64;
    builder.transforms.y.offset = offset.offset_y as f64;
    builder.transforms.z.offset = 0.0;
    builder.vlrs.extend(build_geotiff_crs_vlrs(offset.epsg)?);

    let offset_x = offset.offset_x as f64;
    let offset_y = offset.offset_y as f64;
    // 点格式 3 要求所有点带颜色；PCD 无 rgb 字段时与原 Python 工具一致写 0。
    let has_any_color = points.iter().any(|p| p.color.is_some());
    let las_points: Vec<Point> = points
        .iter()
        .map(|p| {
            let mut point = Point::default();
            point.x = p.x + offset_x;
            point.y = p.y + offset_y;
            point.z = p.z;
            point.gps_time = Some(0.0);
            let (red, green, blue) = if has_any_color {
                p.color.unwrap_or((0, 0, 0))
            } else {
                (0, 0, 0)
            };
            point.color = Some(Color { red, green, blue });
            point
        })
        .collect();

    let mut header = builder.into_header().context("构建 LAS header 失败")?;
    header.clear();
    for point in &las_points {
        header.add_point(point);
    }

    let file = File::create(path).with_context(|| format!("无法创建输出文件: {}", path.display()))?;
    let buffer = BufWriter::with_capacity(16 * 1024 * 1024, file);
    let mut writer = Writer::new(buffer, header)
        .with_context(|| format!("无法创建 LAS writer: {}", path.display()))?;

    const CHUNK: usize = 250_000;
    for chunk in las_points.chunks(CHUNK) {
        writer
            .write_points(chunk)
            .with_context(|| format!("写出点记录失败: {}", path.display()))?;
    }
    Ok(())
}

/// GeoTIFF GeoKey CRS VLR（与项目内其它点云工具一致）。
fn build_geotiff_crs_vlrs(epsg: u16) -> Result<Vec<Vlr>> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let citation = format!(
        "WGS 84 / UTM zone {}{}",
        zone,
        if northern { "N" } else { "S" }
    );
    let ascii_bytes = citation.as_bytes();
    let ascii_len: u16 = ascii_bytes
        .len()
        .try_into()
        .context("GeoTIFF citation 过长")?;
    let mut geokey_data = Vec::with_capacity(32);
    for value in [
        1_u16, 1, 0, 3, 1024, 0, 1, 1, 3072, 0, 1, epsg, 3073, 34737, ascii_len, 0,
    ] {
        geokey_data.extend_from_slice(&value.to_le_bytes());
    }
    Ok(vec![
        Vlr {
            user_id: "LASF_Projection".to_string(),
            record_id: 34735,
            description: "GeoTIFF GeoKeyDirectoryTag".to_string(),
            data: geokey_data,
        },
        Vlr {
            user_id: "LASF_Projection".to_string(),
            record_id: 34737,
            description: "GeoTIFF GeoAsciiParamsTag".to_string(),
            data: ascii_bytes.to_vec(),
        },
    ])
}

fn epsg_to_utm_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("目前仅支持 WGS84 UTM EPSG:32601-32660 或 EPSG:32701-32760"),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    fn write_test_pcd(path: &Path, points: &[(f32, f32, f32, u32)]) {
        let mut header = String::new();
        header.push_str("# .PCD v0.7 - Point Cloud Data file format\n");
        header.push_str("VERSION 0.7\n");
        header.push_str("FIELDS x y z rgb\n");
        header.push_str("SIZE 4 4 4 4\n");
        header.push_str("TYPE F F F F\n");
        header.push_str("COUNT 1 1 1 1\n");
        header.push_str(&format!("WIDTH {}\n", points.len()));
        header.push_str("HEIGHT 1\n");
        header.push_str("VIEWPOINT 0 0 0 1 0 0 0\n");
        header.push_str(&format!("POINTS {}\n", points.len()));
        header.push_str("DATA binary\n");
        let mut file = File::create(path).unwrap();
        file.write_all(header.as_bytes()).unwrap();
        for &(x, y, z, rgb) in points {
            let rgb_f32 = f32::from_bits(rgb);
            file.write_all(&x.to_le_bytes()).unwrap();
            file.write_all(&y.to_le_bytes()).unwrap();
            file.write_all(&z.to_le_bytes()).unwrap();
            file.write_all(&rgb_f32.to_le_bytes()).unwrap();
        }
    }

    #[test]
    fn reads_points_and_packed_colors() {
        let dir = std::env::temp_dir().join("restore_pcd_by_mgrs_test_read");
        std::fs::create_dir_all(&dir).unwrap();
        let pcd_path = dir.join("50QKL416457.pcd");
        // RGB888: R=0x10, G=0x20, B=0x30
        write_test_pcd(&pcd_path, &[(1.5, 2.5, 3.5, 0x102030)]);

        let points = read_pcd_points(&pcd_path).unwrap();
        assert_eq!(points.len(), 1);
        assert_eq!(points[0].x, 1.5);
        assert_eq!(points[0].y, 2.5);
        assert_eq!(points[0].z, 3.5);
        let (r, g, b) = points[0].color.unwrap();
        assert_eq!((r, g, b), (0x10 * 257, 0x20 * 257, 0x30 * 257));
    }

    #[test]
    fn writes_las_with_offset_and_color() {
        let dir = std::env::temp_dir().join("restore_pcd_by_mgrs_test_write");
        std::fs::create_dir_all(&dir).unwrap();
        let las_path = dir.join("out.las");

        let offset = MgrsUtmOffset {
            epsg: 32650,
            offset_x: 341_000,
            offset_y: 2_445_000,
        };
        let points = vec![
            CloudPoint {
                x: 1.5,
                y: 2.5,
                z: 3.5,
                color: Some((0x10 * 257, 0x20 * 257, 0x30 * 257)),
            },
            CloudPoint {
                x: 10.25,
                y: 20.5,
                z: 30.75,
                color: None,
            },
        ];
        write_las(&las_path, &points, &offset).unwrap();

        let mut reader = las::Reader::from_path(&las_path).unwrap();
        let header = reader.header();
        assert_eq!(header.point_format().to_u8().unwrap(), 3);
        assert_eq!(header.transforms().x.scale, 0.01);
        assert_eq!(header.transforms().x.offset, 341_000.0);
        assert_eq!(header.transforms().y.offset, 2_445_000.0);

        let mut read_back = Vec::new();
        while let Some(point) = reader.points().next() {
            let point = point.unwrap();
            read_back.push((point.x, point.y, point.z, point.color));
        }
        assert_eq!(read_back.len(), 2);
        // scale 0.01 量化误差在 ±0.01 内
        assert!((read_back[0].0 - 341_001.5).abs() <= 0.01);
        assert!((read_back[0].1 - 2_445_002.5).abs() <= 0.01);
        assert!((read_back[0].2 - 3.5).abs() <= 0.01);
        let color = read_back[0].3.unwrap();
        assert_eq!((color.red, color.green, color.blue), (0x10 * 257, 0x20 * 257, 0x30 * 257));
        assert!((read_back[1].0 - 341_010.25).abs() <= 0.01);
        // 格式 3 读回必带颜色；文件内有点带色时，无色点补 (0,0,0)
        let color = read_back[1].3.unwrap();
        assert_eq!((color.red, color.green, color.blue), (0, 0, 0));
    }
}
