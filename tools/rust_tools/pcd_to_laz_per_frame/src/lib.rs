mod pcd;
mod transform;

use anyhow::{Context, Result, bail};
use clap::Parser;
use pcd::PcdFile;
use rayon::prelude::*;
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use transform::TransformConfig;

#[derive(Parser)]
#[command(name = "pcd_to_laz_per_frame", version, about = "逐帧 PCD 转 LAZ，可选 ENU 位姿 + origin 偏转")]
struct Cli {
    /// PCD 输入目录（不递归子目录）
    #[arg(short, long, value_name = "DIR")]
    input: PathBuf,

    /// LAZ 输出目录
    #[arg(short, long, value_name = "DIR")]
    output: PathBuf,

    /// origin 原点文件（lat lon alt），不传则不偏转
    #[arg(short = 'O', long, value_name = "FILE")]
    origin: Option<PathBuf>,

    /// ENU 位姿文件（opti_pose_enu.txt），用于将 LiDAR 局部坐标转为世界 ENU
    #[arg(short = 'e', long, value_name = "FILE")]
    enu: Option<PathBuf>,

    /// 线程数
    #[arg(short, long, default_value_t = 4)]
    threads: usize,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = Cli::try_parse_from(
        std::iter::once("pcd_to_laz_per_frame").chain(args.iter().map(String::as_str)),
    )?;

    if !cli.input.is_dir() {
        bail!("--input 不是目录: {}", cli.input.display());
    }
    fs::create_dir_all(&cli.output)
        .with_context(|| format!("无法创建输出目录: {}", cli.output.display()))?;

    let transform = cli
        .origin
        .as_deref()
        .map(|path| transform::build_config(path, None, 0.0))
        .transpose()?;

    let epsg = transform.as_ref().map(|t| t.epsg);

    // 加载 ENU 位姿
    let enu_poses = if let Some(ref enu_path) = cli.enu {
        let poses = pcd::load_enu_poses(enu_path)?;
        eprintln!("加载 {} 条 ENU 位姿", poses.len());
        Some(poses)
    } else {
        None
    };

    // 扫描 PCD 文件（不递归）
    let mut pcd_paths: Vec<PathBuf> = Vec::new();
    for entry in fs::read_dir(&cli.input)
        .with_context(|| format!("无法读取目录: {}", cli.input.display()))?
    {
        let entry = entry?;
        if !entry.file_type()?.is_file() {
            continue;
        }
        let path = entry.path();
        if path.extension().and_then(|e| e.to_str()) == Some("pcd") {
            pcd_paths.push(path);
        }
    }
    pcd_paths.sort();

    if pcd_paths.is_empty() {
        bail!("未找到任何 PCD 文件: {}", cli.input.display());
    }

    // 构建时间戳 -> ENU 位姿映射
    let timestamp_pose: Option<HashMap<String, &pcd::PoseSample>> = enu_poses.as_ref().map(|poses| {
        poses
            .iter()
            .map(|p| (p.timestamp_text.clone(), p))
            .collect()
    });

    // 匹配帧：PCD 文件名 stem 必须存在于 ENU 时间戳中（如果提供了 ENU）
    let mut frames: Vec<(PathBuf, Option<&pcd::PoseSample>)> = Vec::new();
    let mut unmatched = 0usize;
    for pcd_path in &pcd_paths {
        let stem = pcd_path
            .file_stem()
            .and_then(|s| s.to_str())
            .unwrap_or("");
        match &timestamp_pose {
            Some(map) => {
                if let Some(pose) = map.get(stem) {
                    frames.push((pcd_path.clone(), Some(pose)));
                } else {
                    unmatched += 1;
                    eprintln!("SKIP  {} （ENU 中无匹配位姿）", pcd_path.display());
                }
            }
            None => {
                frames.push((pcd_path.clone(), None));
            }
        }
    }

    eprintln!("找到 {} 个 PCD，匹配 {} 帧，跳过 {} 帧",
        pcd_paths.len(), frames.len(), unmatched);
    if let Some(ref t) = transform {
        eprintln!(
            "偏转模式: EPSG:{} origin=({:.6},{:.6},{:.2})",
            t.epsg, t.origin.lat, t.origin.lon, t.origin.alt
        );
    } else {
        eprintln!("无偏转模式");
    }
    if cli.enu.is_some() {
        eprintln!("ENU 位姿: 已启用");
    }

    if frames.is_empty() {
        bail!("没有可处理的帧");
    }

    if cli.threads > 0 {
        rayon::ThreadPoolBuilder::new()
            .num_threads(cli.threads)
            .build_global()
            .context("初始化线程池失败")?;
    }

    let results: Vec<Result<String>> = frames
        .par_iter()
        .map(|(pcd_path, pose)| process_one(pcd_path, &cli.output, *pose, transform.as_ref(), epsg))
        .collect();

    let mut ok = 0usize;
    let mut fail = 0usize;
    for result in &results {
        match result {
            Ok(msg) => {
                ok += 1;
                eprintln!("  OK  {msg}");
            }
            Err(e) => {
                fail += 1;
                eprintln!("FAIL  {e:#}");
            }
        }
    }
    eprintln!("完成: 成功 {ok}, 失败 {fail}");
    if ok == 0 && fail > 0 {
        bail!("全部失败");
    }
    Ok(())
}

fn process_one(
    pcd_path: &std::path::Path,
    output_dir: &std::path::Path,
    pose: Option<&pcd::PoseSample>,
    transform: Option<&TransformConfig>,
    epsg: Option<u16>,
) -> Result<String> {
    let stem = pcd_path
        .file_stem()
        .and_then(|s| s.to_str())
        .context("无法获取文件名")?;
    let laz_path = output_dir.join(format!("{stem}.laz"));

    let pcd_file = PcdFile::open(pcd_path)?;
    let points = pcd_file.load_points(pose, transform)?;

    pcd::write_laz(&laz_path, &points, epsg)?;
    Ok(format!("{stem}.pcd -> {stem}.laz ({} 点)", points.len()))
}
