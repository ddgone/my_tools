use crate::legacy::logging::{ProcessLogger, log_process};
use crate::legacy::types::{
    FrameEntry, ParsedPcdHeader, PcdCandidate, PcdField, PcdFieldKind, PcdSchema, PoseSample,
};
use anyhow::{Context, Result, bail};
use byteorder::{ByteOrder, LittleEndian};
use glam::{DQuat, DVec3};
use las::{Builder, Header, Point, point::Format};
use std::collections::{HashMap, HashSet};
use std::fs::{self, File};
use std::io::{BufRead, BufReader, Read, Seek, SeekFrom};
use std::path::Path;
use std::sync::{Arc, Mutex};

pub(crate) struct PcdScanResult {
    pub(crate) candidates: HashMap<String, PcdCandidate>,
    pub(crate) scanned_pcd_files: usize,
    pub(crate) valid_pcd_files: usize,
    pub(crate) skipped_pcd_files: usize,
}

pub(crate) fn load_pcd_poses(path: &Path) -> Result<Vec<PoseSample>> {
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

        let tx = parse_pcd_f64(fields[1], path, line_no + 1, "tx")?;
        let ty = parse_pcd_f64(fields[2], path, line_no + 1, "ty")?;
        let tz = parse_pcd_f64(fields[3], path, line_no + 1, "tz")?;
        let qx = parse_pcd_f64(fields[4], path, line_no + 1, "qx")?;
        let qy = parse_pcd_f64(fields[5], path, line_no + 1, "qy")?;
        let qz = parse_pcd_f64(fields[6], path, line_no + 1, "qz")?;
        let qw = parse_pcd_f64(fields[7], path, line_no + 1, "qw")?;
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

fn parse_pcd_f64(value: &str, path: &Path, line_no: usize, field_name: &str) -> Result<f64> {
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

pub(crate) fn scan_pcd_candidates(
    path: &Path,
    logger: &Arc<Mutex<ProcessLogger>>,
) -> Result<PcdScanResult> {
    let mut candidates = HashMap::<String, PcdCandidate>::new();
    let mut scanned_pcd_files = 0usize;
    let mut valid_pcd_files = 0usize;
    let mut skipped_pcd_files = 0usize;
    let mut entries = fs::read_dir(path)
        .with_context(|| format!("无法读取 PCD 目录: {}", path.display()))?
        .collect::<std::result::Result<Vec<_>, _>>()
        .with_context(|| format!("枚举 PCD 目录失败: {}", path.display()))?;
    entries.sort_by_key(|entry| entry.path());

    for entry in entries {
        if !entry.file_type()?.is_file() {
            continue;
        }
        let file_path = entry.path();
        if file_path.extension().and_then(|ext| ext.to_str()) != Some("pcd") {
            continue;
        }
        scanned_pcd_files += 1;
        let timestamp_text = match file_path.file_stem().and_then(|stem| stem.to_str()) {
            Some(stem) => stem.to_string(),
            None => {
                skipped_pcd_files += 1;
                log_process(
                    logger,
                    "WARN",
                    format!("跳过无法解析文件名的 PCD: {}", file_path.display()),
                );
                continue;
            }
        };
        let timestamp = match timestamp_text.parse::<f64>() {
            Ok(value) => value,
            Err(_) => {
                skipped_pcd_files += 1;
                log_process(
                    logger,
                    "WARN",
                    format!("跳过时间戳文件名非法的 PCD: {}", file_path.display()),
                );
                continue;
            }
        };
        let metadata = entry.metadata()?;
        if metadata.len() == 0 {
            skipped_pcd_files += 1;
            log_process(
                logger,
                "WARN",
                format!("跳过空文件 PCD: {}", file_path.display()),
            );
            continue;
        }
        let header = match parse_pcd_header_file(&file_path) {
            Ok(header) => header,
            Err(error) => {
                skipped_pcd_files += 1;
                log_process(
                    logger,
                    "WARN",
                    format!("跳过头解析失败的 PCD {}: {error:#}", file_path.display()),
                );
                continue;
            }
        };
        if header.data_encoding != "binary" {
            skipped_pcd_files += 1;
            log_process(
                logger,
                "WARN",
                format!(
                    "跳过非 binary PCD {}: DATA={}",
                    file_path.display(),
                    header.data_encoding
                ),
            );
            continue;
        }
        if header.points == 0 {
            skipped_pcd_files += 1;
            log_process(
                logger,
                "WARN",
                format!("跳过无点数据的 PCD: {}", file_path.display()),
            );
            continue;
        }
        if candidates.contains_key(&timestamp_text) {
            skipped_pcd_files += 1;
            log_process(
                logger,
                "WARN",
                format!("跳过重复时间戳 PCD: {}", file_path.display()),
            );
            continue;
        }
        valid_pcd_files += 1;
        candidates.insert(
            timestamp_text,
            PcdCandidate {
                timestamp,
                path: file_path,
                point_count: header.points,
                data_offset: header.data_offset,
                schema: header.schema,
            },
        );
    }

    Ok(PcdScanResult {
        candidates,
        scanned_pcd_files,
        valid_pcd_files,
        skipped_pcd_files,
    })
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
                    .with_context(|| format!("PCD SIZE 字段非法: {}", path.display()))?;
            }
            "TYPE" => {
                kinds = values
                    .iter()
                    .map(|value| parse_field_kind(value, path))
                    .collect::<Result<_>>()?;
            }
            "COUNT" => {
                counts = values
                    .iter()
                    .map(|value| value.parse::<usize>())
                    .collect::<std::result::Result<_, _>>()
                    .with_context(|| format!("PCD COUNT 字段非法: {}", path.display()))?;
            }
            "POINTS" => {
                points = Some(
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

fn parse_field_kind(value: &str, path: &Path) -> Result<PcdFieldKind> {
    match value {
        "I" => Ok(PcdFieldKind::Signed),
        "U" => Ok(PcdFieldKind::Unsigned),
        "F" => Ok(PcdFieldKind::Float),
        _ => bail!("PCD TYPE {:?} 不支持: {}", value, path.display()),
    }
}

pub(crate) fn load_and_transform_frame(
    frame: &FrameEntry,
    pose: &PoseSample,
) -> Result<Vec<Point>> {
    let mut file = File::open(&frame.path)
        .with_context(|| format!("无法打开 PCD: {}", frame.path.display()))?;
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
        let local = DVec3::new(
            frame
                .schema
                .x_field
                .scalar_as_f64(record)
                .context("x 字段无效")?,
            frame
                .schema
                .y_field
                .scalar_as_f64(record)
                .context("y 字段无效")?,
            frame
                .schema
                .z_field
                .scalar_as_f64(record)
                .context("z 字段无效")?,
        );
        let world = apply_pose(local, pose);
        let mut point = Point::default();
        point.x = world.x;
        point.y = world.y;
        point.z = world.z;
        point.intensity = frame
            .schema
            .intensity_field
            .and_then(|field| field.scalar_as_f64(record))
            .map(quantize_intensity_raw)
            .unwrap_or(0);
        points.push(point);
    }
    Ok(points)
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

fn apply_pose(local: DVec3, pose: &PoseSample) -> DVec3 {
    pose.translation + pose.rotation * local
}

fn quantize_intensity_raw(value: f64) -> u16 {
    if !value.is_finite() || value <= 0.0 {
        0
    } else {
        value.round().clamp(0.0, 65535.0) as u16
    }
}

pub(crate) fn build_pcd_header_template() -> Result<Header> {
    let mut builder = Builder::from((1, 2));
    builder.generating_software = "bxn_delivery_point_cloud_qc".to_string();
    builder.system_identifier = "pcd_stream".to_string();
    builder.point_format = Format::new(0).context("创建 LAS 点格式失败")?;
    builder.transforms.x.scale = 0.001;
    builder.transforms.y.scale = 0.001;
    builder.transforms.z.scale = 0.001;
    builder.into_header().context("创建 PCD 输出头失败")
}
