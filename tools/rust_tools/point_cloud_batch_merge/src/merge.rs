use std::fs::{self, File};
use std::io::BufWriter;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;

use anyhow::{Context, Result};
use flate2::read::GzDecoder;
use las::{Builder, Point, Writer};
use rayon::prelude::*;
use tar::Archive;

use crate::cli::OutputFormat;
use crate::pcd::{self, build_rotation, PcdFrame};
use crate::pos::{self, PosPose};

struct Package {
    name: String,
    pcd_dir: PathBuf,
    pos_path: PathBuf,
}

pub fn run(cli: &crate::cli::Cli) -> Result<()> {
    fs::create_dir_all(&cli.output)
        .with_context(|| format!("无法创建输出目录: {}", cli.output.display()))?;

    rayon::ThreadPoolBuilder::new()
        .num_threads(cli.threads)
        .build_global()
        .ok();

    let temp_root = cli.output.join(".pcd_merge_tmp");
    let packages = discover_packages(&cli.input, &cli.pos_dir, &temp_root)?;
    if packages.is_empty() {
        anyhow::bail!("未找到可处理的数据包");
    }

    let total = packages.len();
    let failed_count = AtomicU64::new(0);

    for (index, package) in packages.iter().enumerate() {
        let output_path = cli.output.join(format!("{}.{}", package.name, cli.format.extension()));
        eprintln!(
            "[{}/{}] 开始: {} -> {}",
            index + 1, total, package.name, output_path.display()
        );
        match process_package(package, &output_path, cli.format, cli.threads, cli.flip_z) {
            Ok(stats) => {
                eprintln!(
                    "[{}/{}] 完成: {}，匹配 {} 帧，总点数 {}",
                    index + 1, total, package.name, stats.matched_frames, stats.total_points,
                );
            }
            Err(err) => {
                failed_count.fetch_add(1, Ordering::Relaxed);
                eprintln!("[{}/{}] 失败: {} - {err:#}", index + 1, total, package.name);
            }
        }
    }

    let _ = fs::remove_dir_all(&temp_root);

    let failed = failed_count.load(Ordering::Relaxed);
    let success = total - failed as usize;
    eprintln!("全部完成: 成功 {}，失败 {}，总计 {}", success, failed, total);
    if success == 0 {
        anyhow::bail!("没有任何数据包成功输出");
    }
    Ok(())
}

struct ProcessStats {
    matched_frames: usize,
    total_points: u64,
}

fn process_package(
    package: &Package,
    output_path: &Path,
    format: OutputFormat,
    threads: usize,
    flip_z: bool,
) -> Result<ProcessStats> {
    let poses = pos::load_pos(&package.pos_path)?;
    if poses.is_empty() {
        anyhow::bail!("POS 数据为空");
    }

    let pcd_frames = pcd::scan_pcd_frames(&package.pcd_dir)?;
    if pcd_frames.is_empty() {
        anyhow::bail!("未找到有效 PCD 文件");
    }

    // 匹配帧（PCD 时间戳 -> POS 时间戳）
    let mut matched: Vec<(&PcdFrame, &PosPose)> = Vec::new();
    let mut pcd_unmatched = 0usize;
    for (&ts, frame) in &pcd_frames {
        if let Some(pose) = poses.get(&ts) {
            matched.push((frame, pose));
        } else {
            pcd_unmatched += 1;
        }
    }
    matched.sort_by_key(|(f, _)| f.timestamp_ms);
    if matched.is_empty() {
        anyhow::bail!("没有 PCD 帧与 POS 时间戳匹配");
    }
    eprintln!(
        "  匹配: {}/{} 帧，PCD 无 POS: {}，POS 多余: {}",
        matched.len(),
        pcd_frames.len(),
        pcd_unmatched,
        poses.len().saturating_sub(matched.len()),
    );

    // 确定 UTM 投影带
    let first_pose = &matched[0].1;
    let epsg = crate::utm::infer_epsg(first_pose.y, first_pose.x);
    eprintln!("  UTM EPSG:{}", epsg);

    // 并行加载帧：旋转 → WGS84 → UTM
    let total_points = AtomicU64::new(0);
    let all_points_mutex = Mutex::new(Vec::new());

    let chunk_size = matched.len().div_ceil(threads).max(1);
    for chunk in matched.chunks(chunk_size) {
        let results: Vec<Result<Vec<Point>>> = chunk
            .par_iter()
            .map(|(frame, pose)| {
                let local_points = pcd::load_frame_raw(frame)?;
                let rot = build_rotation(pose.azimuth_deg);
                let origin_lat = pose.y;
                let origin_lon = pose.x;
                let origin_alt = pose.z;
                let world_points: Vec<Point> = local_points
                    .into_iter()
                    .map(|mut p| {
                        let enu = rot * glam::DVec3::new(p.x, p.y, p.z);
                        let enu_z = if flip_z { -enu.z } else { enu.z };
                        // ENU(m) → WGS84(lon,lat,alt) via ECEF（精密测地线）
                        let (lon, lat, alt) = crate::utm::enu_to_wgs84(
                            enu.x, enu.y, enu_z, origin_lat, origin_lon, origin_alt,
                        );
                        // WGS84 → UTM
                        if let Ok((e, n, a)) = crate::utm::wgs84_to_utm(lat, lon, alt, epsg) {
                            p.x = e;
                            p.y = n;
                            p.z = a;
                        } else {
                            p.x = lon;
                            p.y = lat;
                            p.z = alt;
                        }
                        p
                    })
                    .collect();
                total_points.fetch_add(world_points.len() as u64, Ordering::Relaxed);
                Ok(world_points)
            })
            .collect();

        for result in results {
            all_points_mutex.lock().unwrap().extend(result?);
        }
    }

    let all_points = all_points_mutex.into_inner().unwrap();
    if all_points.is_empty() {
        anyhow::bail!("没有有效点数据可输出");
    }

    write_las(output_path, &all_points, format, epsg)?;

    Ok(ProcessStats {
        matched_frames: matched.len(),
        total_points: total_points.load(Ordering::Relaxed),
    })
}

fn write_las(path: &Path, points: &[Point], format: OutputFormat, epsg: u16) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }

    let (min_x, min_y, min_z, max_x, max_y, max_z) = compute_bbox(points);

    let mut builder = Builder::from((1, 2));
    builder.generating_software = "point_cloud_batch_merge".to_string();
    builder.system_identifier = "pcd_merge".to_string();
    builder.point_format = las::point::Format::new(0).context("创建 LAS 点格式失败")?;
    // UTM 米坐标：0.001m (mm) 精度
    builder.transforms.x.scale = 0.001;
    builder.transforms.y.scale = 0.001;
    builder.transforms.z.scale = 0.001;
    builder.transforms.x.offset = min_x.floor();
    builder.transforms.y.offset = min_y.floor();
    builder.transforms.z.offset = min_z.floor();
    builder.vlrs.extend(crate::utm::build_utm_vlrs(epsg)?);

    let compress = matches!(format, OutputFormat::Laz);
    builder.point_format.is_compressed = compress;

    let header = builder.into_header().context("构建输出 LAS header 失败")?;
    let file = File::create(path)?;
    let buffer = BufWriter::with_capacity(16 * 1024 * 1024, file);
    let mut writer = Writer::new(buffer, header)?;

    for chunk in points.chunks(250_000) {
        writer.write_points(chunk)?;
    }

    drop(writer);
    eprintln!(
        "  bbox: E=[{:.1},{:.1}] N=[{:.1},{:.1}] alt=[{:.1},{:.1}]",
        min_x, max_x, min_y, max_y, min_z, max_z
    );
    Ok(())
}

fn compute_bbox(points: &[Point]) -> (f64, f64, f64, f64, f64, f64) {
    let mut min_x = f64::INFINITY;
    let mut min_y = f64::INFINITY;
    let mut min_z = f64::INFINITY;
    let mut max_x = f64::NEG_INFINITY;
    let mut max_y = f64::NEG_INFINITY;
    let mut max_z = f64::NEG_INFINITY;
    for p in points {
        min_x = min_x.min(p.x);
        min_y = min_y.min(p.y);
        min_z = min_z.min(p.z);
        max_x = max_x.max(p.x);
        max_y = max_y.max(p.y);
        max_z = max_z.max(p.z);
    }
    (min_x, min_y, min_z, max_x, max_y, max_z)
}

fn discover_packages(cloud_dir: &Path, pos_dir: &Path, temp_root: &Path) -> Result<Vec<Package>> {
    let mut packages = Vec::new();
    let mut entries: Vec<_> = fs::read_dir(cloud_dir)?.collect::<std::result::Result<Vec<_>, _>>()?;
    entries.sort_by_key(|entry| entry.path());

    for entry in entries {
        if !entry.file_type()?.is_file() {
            continue;
        }
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("gz") {
            continue;
        }
        let file_name = path.file_stem().and_then(|s| s.to_str()).unwrap_or("");
        if !file_name.ends_with(".tar") {
            continue;
        }
        let pkg_name = &file_name[..file_name.len() - 4];
        let track_id = pkg_name.split('-').next().unwrap_or(pkg_name);
        let pos_path = pos_dir.join(format!("{track_id}.json"));
        if !pos_path.is_file() {
            eprintln!("警告: 找不到 POS 文件 {}，跳过 {}", pos_path.display(), pkg_name);
            continue;
        }

        let extract_dir = temp_root.join(pkg_name);
        let pcd_dir = extract_tar_gz(&path, &extract_dir, pkg_name)?;
        packages.push(Package { name: pkg_name.to_string(), pcd_dir, pos_path });
    }
    Ok(packages)
}

fn extract_tar_gz(archive_path: &Path, dest_dir: &Path, pkg_name: &str) -> Result<PathBuf> {
    if dest_dir.exists() {
        fs::remove_dir_all(dest_dir)?;
    }
    fs::create_dir_all(dest_dir)?;
    let file = File::open(archive_path)?;
    let decoder = GzDecoder::new(file);
    let mut archive = Archive::new(decoder);
    archive.unpack(dest_dir)?;
    find_pcd_dir(dest_dir, pkg_name)
}

fn find_pcd_dir(root: &Path, pkg_name: &str) -> Result<PathBuf> {
    if has_pcd_files(root) {
        return Ok(root.to_path_buf());
    }
    let mut stack = vec![root.to_path_buf()];
    while let Some(dir) = stack.pop() {
        for entry in fs::read_dir(&dir)? {
            let path = entry?.path();
            if path.is_dir() {
                if has_pcd_files(&path) {
                    return Ok(path);
                }
                stack.push(path);
            }
        }
    }
    anyhow::bail!("解压后未找到 .pcd 文件: {}", pkg_name);
}

fn has_pcd_files(dir: &Path) -> bool {
    fs::read_dir(dir).map_or(false, |entries| {
        entries.filter_map(|e| e.ok()).any(|e| {
            e.path().extension().and_then(|ext| ext.to_str())
                .map(|ext| ext.eq_ignore_ascii_case("pcd"))
                .unwrap_or(false)
        })
    })
}
