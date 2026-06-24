use anyhow::{Context, Result, bail};
use clap::{Parser, ValueEnum};
use image::{ImageBuffer, Rgba};
use las::{Builder, Header, Point, Reader, Vlr, Writer, crs::GeoTiffData, laz};
use rayon::{ThreadPoolBuilder, prelude::*};
use rustc_hash::FxHashMap;
use std::collections::hash_map::Entry;
use std::f64::consts::PI;
use std::fs;
use std::path::{Component, Path, PathBuf};
use std::time::Instant;

const WGS84_A: f64 = 6_378_137.0;
const WGS84_F: f64 = 1.0 / 298.257_223_563;
const UTM_K0: f64 = 0.9996;
const UTM_FALSE_EASTING: f64 = 500_000.0;
const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;

#[derive(Debug, Parser)]
#[command(
    author,
    version,
    about = "对 LAS/LAZ 执行批量抽稀、偏转和强度图输出"
)]
struct BatchCli {
    #[arg(short, long, value_name = "PATH", required = true)]
    input: Vec<PathBuf>,

    #[arg(short, long, value_name = "DIR")]
    output: PathBuf,

    #[arg(long)]
    thin: bool,

    #[arg(long)]
    rotate: bool,

    #[arg(long = "intensity-map")]
    intensity_map: bool,

    #[arg(long, default_value_t = 0.1)]
    voxel_size: f64,

    #[arg(long, value_enum, default_value_t = RepresentativeMode::Center)]
    representative: RepresentativeMode,

    #[arg(long, value_name = "COUNT")]
    reserve: Option<usize>,

    #[arg(long, value_name = "N")]
    threads: Option<usize>,

    #[arg(long, value_name = "METERS")]
    intensity_resolution: Option<f64>,

    #[arg(long, default_value_t = 0.0)]
    yaw_deg: f64,

    #[arg(long, value_enum, default_value_t = PivotMode::Centroid)]
    pivot: PivotMode,

    #[arg(long, value_enum, default_value_t = MappingMode::Enu)]
    mapping: MappingMode,

    #[arg(long, value_name = "EPSG")]
    epsg: Option<u16>,

    #[arg(long)]
    force: bool,

    #[arg(long)]
    quiet: bool,
}

#[derive(Debug, Clone)]
struct Cli {
    input: PathBuf,
    output: Option<PathBuf>,
    voxel_size: f64,
    representative: RepresentativeMode,
    reserve: Option<usize>,
    threads: Option<usize>,
    intensity_preview: Option<PathBuf>,
    intensity_resolution: Option<f64>,
    origin: Option<PathBuf>,
    yaw_deg: f64,
    pivot: PivotMode,
    mapping: MappingMode,
    epsg: Option<u16>,
    preprocessed_output: Option<PathBuf>,
    raster_only: bool,
    force: bool,
    quiet: bool,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
enum RepresentativeMode {
    First,
    Center,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
enum PivotMode {
    Centroid,
    BboxCenter,
    Zero,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
enum MappingMode {
    Enu,
    Flat,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, Hash)]
struct VoxelKey {
    x: i32,
    y: i32,
    z: i32,
}

#[derive(Debug)]
struct SelectedPoint {
    point: Point,
    score: f64,
}

#[derive(Debug, Default, Clone)]
struct RasterCell {
    intensity_sum: f64,
    count: u32,
}

#[derive(Debug, Clone)]
struct RasterLayout {
    min_x: f64,
    min_y: f64,
    max_x: f64,
    max_y: f64,
    resolution: f64,
    width: usize,
    height: usize,
    pixel_count: usize,
}

#[derive(Debug, Clone)]
struct OriginInfo {
    lat: f64,
    lon: f64,
    alt: f64,
}

#[derive(Debug, Clone)]
struct TransformConfig {
    origin: OriginInfo,
    epsg: u16,
    origin_utm: (f64, f64, f64),
    origin_ecef: (f64, f64, f64),
    pivot: (f64, f64),
    yaw_deg: f64,
    mapping: MappingMode,
}

#[derive(Debug, Clone)]
struct TransformSummary {
    source: PathBuf,
    point_count: u64,
    epsg: u16,
    origin_utm: (f64, f64, f64),
    pivot: (f64, f64),
    yaw_deg: f64,
    mapping: MappingMode,
}

#[derive(Debug, Clone)]
struct PivotAccumulator {
    count: u64,
    sum_x: f64,
    sum_y: f64,
    min_x: f64,
    min_y: f64,
    max_x: f64,
    max_y: f64,
}

#[derive(Debug)]
struct PreparedPointCloud {
    header: Header,
    points: Vec<Point>,
    summary: TransformSummary,
}

#[derive(Debug, Clone, Copy, Eq, PartialEq)]
enum JobMode {
    Standard,
    TransformOnly,
}

#[derive(Debug, Clone)]
struct JobPlan {
    cli: Cli,
    mode: JobMode,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = BatchCli::parse_from(args);
    validate_batch_args(&cli)?;
    configure_threads(cli.threads)?;

    let jobs = build_job_plans(&cli)?;
    if jobs.is_empty() {
        bail!("没有找到可处理的 .las/.laz 文件");
    }

    if !cli.quiet {
        eprintln!(
            "准备处理 {} 个点云文件，能力: 抽稀={}, 偏转={}, 强度图={}",
            jobs.len(),
            cli.thin,
            cli.rotate,
            cli.intensity_map
        );
    }

    for (index, job) in jobs.iter().enumerate() {
        if !cli.quiet {
            eprintln!(
                "[{}/{}] 开始处理 {}",
                index + 1,
                jobs.len(),
                job.cli.input.display()
            );
        }
        validate_args(&job.cli)?;
        let start = Instant::now();
        match job.mode {
            JobMode::Standard => run_standard_job(&job.cli, start)?,
            JobMode::TransformOnly => run_transform_only(&job.cli, start)?,
        }
    }

    Ok(())
}

fn run_standard_job(cli: &Cli, start: Instant) -> Result<()> {
    let mut reader = Reader::from_path(&cli.input)
        .with_context(|| format!("无法打开输入文件: {}", cli.input.display()))?;
    let header = reader.header().clone();

    if cli.origin.is_some() {
        if cli.raster_only || cli.preprocessed_output.is_some() {
            let prepared = preprocess_point_cloud(&cli, &mut reader, &header, start)?;
            if cli.raster_only {
                run_raster_only_prepared(&cli, &prepared.header, &prepared.points, start, &prepared.summary)?;
            } else {
                run_voxelize_prepared(&cli, &prepared.header, &prepared.points, start, &prepared.summary)?;
            }
        } else {
            run_voxelize_with_late_transform(&cli, &mut reader, &header, start)?;
        }
    } else if cli.raster_only {
        run_raster_only(&cli, &mut reader, &header, start)?;
    } else {
        run_voxelize(&cli, &mut reader, &header, start)?;
    }

    Ok(())
}

fn run_transform_only(cli: &Cli, start: Instant) -> Result<()> {
    let output_path = cli
        .output
        .as_ref()
        .context("启用偏转且未开启抽稀时必须指定输出点云路径")?;
    let origin_path = cli
        .origin
        .as_ref()
        .context("启用偏转时必须提供 origin.txt")?;

    let mut reader = Reader::from_path(&cli.input)
        .with_context(|| format!("无法打开输入文件: {}", cli.input.display()))?;
    let header = reader.header().clone();
    let prepared = preprocess_point_cloud(cli, &mut reader, &header, start)?;

    if !cli.quiet {
        eprintln!(
            "开始写出偏转点云: input={}, output={}, origin={}",
            cli.input.display(),
            output_path.display(),
            origin_path.display()
        );
        log_transform_summary(&prepared.summary);
    }

    write_point_cloud(output_path, &prepared.header, &prepared.points, cli.force)
        .with_context(|| format!("无法创建输出文件: {}", output_path.display()))?;

    if let Some(preview_path) = &cli.intensity_preview {
        let resolution = cli.intensity_resolution.unwrap_or(cli.voxel_size);
        if !cli.quiet {
            eprintln!(
                "开始输出强度预览图: path={}, resolution={}m",
                preview_path.display(),
                resolution
            );
        }
        write_intensity_preview_points(
            preview_path,
            resolution,
            &prepared.points,
            &prepared.header,
            cli.force,
        )
        .with_context(|| format!("输出强度预览图失败: {}", preview_path.display()))?;
    }

    if !cli.quiet {
        eprintln!(
            "完成偏转输出: {} 点，总耗时 {:.2}s",
            prepared.points.len(),
            start.elapsed().as_secs_f64()
        );
    }

    Ok(())
}

fn validate_batch_args(cli: &BatchCli) -> Result<()> {
    if cli.input.is_empty() {
        bail!("至少提供一个输入路径");
    }
    if !cli.thin && !cli.rotate && !cli.intensity_map {
        bail!("请至少启用一个能力: 抽稀、偏转或强度图");
    }
    if cli.output.exists() && !cli.output.is_dir() {
        bail!("输出路径必须是目录: {}", cli.output.display());
    }
    if !(cli.voxel_size.is_finite() && cli.voxel_size > 0.0) {
        bail!("--voxel-size 必须是正数");
    }
    if !cli.yaw_deg.is_finite() {
        bail!("--yaw-deg 必须是有限数值");
    }
    if let Some(threads) = cli.threads {
        if threads == 0 {
            bail!("--threads 必须大于 0");
        }
    }
    if let Some(resolution) = cli.intensity_resolution {
        if !(resolution.is_finite() && resolution > 0.0) {
            bail!("--intensity-resolution 必须是正数");
        }
    }
    if let Some(epsg) = cli.epsg {
        epsg_to_utm_zone(epsg)?;
    }
    for input in &cli.input {
        if !input.exists() {
            bail!("输入路径不存在: {}", input.display());
        }
    }
    Ok(())
}

fn build_job_plans(cli: &BatchCli) -> Result<Vec<JobPlan>> {
    let mut jobs = Vec::new();
    for input in &cli.input {
        if input.is_file() {
            if !is_las_path(input) {
                bail!("输入文件必须是 .las 或 .laz: {}", input.display());
            }
            if cli.rotate && !cli.quiet {
                eprintln!(
                    "文件输入不会自动偏转，已跳过偏转: {}",
                    input.display()
                );
            }
            jobs.push(build_job_plan(cli, input.clone(), None, false)?);
            continue;
        }

        if input.is_dir() {
            let origin_path = resolve_origin_path(input);
            let files = collect_las_files(input)?;
            if files.is_empty() {
                if !cli.quiet {
                    eprintln!(
                        "目录下未找到当前层级的 .las/.laz 文件，已跳过: {}",
                        input.display()
                    );
                }
                continue;
            }
            if cli.rotate && origin_path.is_none() && !cli.quiet {
                eprintln!(
                    "目录下未找到 origin.txt，已跳过偏转: {}",
                    input.display()
                );
            }
            for file in files {
                jobs.push(build_job_plan(cli, file, origin_path.clone(), true)?);
            }
            continue;
        }

        bail!("输入路径既不是文件也不是目录: {}", input.display());
    }

    Ok(jobs)
}

fn build_job_plan(
    batch: &BatchCli,
    input_file: PathBuf,
    directory_origin: Option<PathBuf>,
    input_is_directory: bool,
) -> Result<JobPlan> {
    let apply_rotation = batch.rotate && input_is_directory && directory_origin.is_some();
    let write_point_cloud = batch.thin || apply_rotation;
    let output_path = if write_point_cloud {
        Some(build_output_point_cloud_path(&batch.output, &input_file)?)
    } else {
        None
    };
    let intensity_preview = if batch.intensity_map {
        Some(build_intensity_preview_path(
            &batch.output,
            &input_file,
            output_path.as_deref(),
        )?)
    } else {
        None
    };

    let cli = Cli {
        input: input_file,
        output: output_path,
        voxel_size: batch.voxel_size,
        representative: batch.representative,
        reserve: batch.reserve,
        threads: batch.threads,
        intensity_preview,
        intensity_resolution: batch.intensity_resolution,
        origin: if apply_rotation { directory_origin } else { None },
        yaw_deg: batch.yaw_deg,
        pivot: batch.pivot,
        mapping: batch.mapping,
        epsg: batch.epsg,
        preprocessed_output: None,
        raster_only: batch.intensity_map && !write_point_cloud,
        force: batch.force,
        quiet: batch.quiet,
    };

    let mode = if apply_rotation && !batch.thin {
        JobMode::TransformOnly
    } else {
        JobMode::Standard
    };

    Ok(JobPlan { cli, mode })
}

fn collect_las_files(dir: &Path) -> Result<Vec<PathBuf>> {
    let mut files = Vec::new();
    for entry in fs::read_dir(dir).with_context(|| format!("无法读取目录: {}", dir.display()))? {
        let entry = entry.with_context(|| format!("无法读取目录项: {}", dir.display()))?;
        let path = entry.path();
        if path.is_file() && is_las_path(&path) {
            files.push(path);
        }
    }
    files.sort();
    Ok(files)
}

fn resolve_origin_path(dir: &Path) -> Option<PathBuf> {
    let path = dir.join("origin.txt");
    if path.is_file() {
        Some(path)
    } else {
        None
    }
}

fn is_las_path(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| matches!(ext.to_ascii_lowercase().as_str(), "las" | "laz"))
        .unwrap_or(false)
}

fn build_output_point_cloud_path(output_root: &Path, input_file: &Path) -> Result<PathBuf> {
    let file_name = input_file
        .file_name()
        .context("无法获取输入文件名以生成输出路径")?;
    let mut output = output_root.to_path_buf();
    let relative_parent = sanitize_parent_path(input_file);
    if !relative_parent.as_os_str().is_empty() {
        output.push(relative_parent);
    }
    output.push(file_name);
    Ok(output)
}

fn build_intensity_preview_path(
    output_root: &Path,
    input_file: &Path,
    point_cloud_output: Option<&Path>,
) -> Result<PathBuf> {
    let base = if let Some(path) = point_cloud_output {
        path.to_path_buf()
    } else {
        build_output_point_cloud_path(output_root, input_file)?
    };
    Ok(base.with_extension("png"))
}

fn sanitize_parent_path(path: &Path) -> PathBuf {
    let mut sanitized = PathBuf::new();
    if let Some(parent) = path.parent() {
        for component in parent.components() {
            match component {
                Component::Prefix(prefix) => sanitized.push(sanitize_component(prefix.as_os_str())),
                Component::RootDir => {}
                Component::CurDir => {}
                Component::ParentDir => sanitized.push("__parent__"),
                Component::Normal(part) => sanitized.push(sanitize_component(part)),
            }
        }
    }
    sanitized
}

fn sanitize_component(value: &std::ffi::OsStr) -> String {
    let raw = value.to_string_lossy();
    let mut result = String::with_capacity(raw.len());
    for ch in raw.chars() {
        match ch {
            '<' | '>' | ':' | '"' | '/' | '\\' | '|' | '?' | '*' => result.push('_'),
            _ => result.push(ch),
        }
    }
    let trimmed = result.trim_matches('.').trim();
    if trimmed.is_empty() {
        "_".to_string()
    } else {
        trimmed.to_string()
    }
}

fn preprocess_point_cloud(
    cli: &Cli,
    reader: &mut Reader,
    header: &Header,
    start: Instant,
) -> Result<PreparedPointCloud> {
    let origin_path = cli
        .origin
        .as_ref()
        .context("启用预处理时必须提供 --origin")?;
    let origin = read_origin(origin_path)?;
    let epsg = cli.epsg.unwrap_or_else(|| infer_utm_epsg(origin.lat, origin.lon));
    let origin_utm = utm_from_geodetic(origin.lat, origin.lon, origin.alt, epsg)?;
    let origin_ecef = ecef_from_geodetic(origin.lat, origin.lon, origin.alt);

    if !cli.quiet {
        eprintln!(
            "开始 origin 预处理: input={}, origin={}, mapping={:?}, yaw={}deg, epsg=EPSG:{}",
            cli.input.display(),
            origin_path.display(),
            cli.mapping,
            cli.yaw_deg,
            epsg
        );
    }

    let mut points = Vec::with_capacity(header.number_of_points() as usize);
    for wrapped_point in reader.points() {
        let point = wrapped_point
            .with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
        points.push(point);
    }

    let pivot = get_pivot(&points, cli.pivot);
    let config = TransformConfig {
        origin,
        epsg,
        origin_utm,
        origin_ecef,
        pivot,
        yaw_deg: cli.yaw_deg,
        mapping: cli.mapping,
    };

    for (idx, point) in points.iter_mut().enumerate() {
        apply_transform(point, &config)?;

        let processed = idx as u64 + 1;
        if !cli.quiet && processed % 5_000_000 == 0 {
            eprintln!(
                "预处理已完成 {} 点，耗时 {:.1}s",
                processed,
                start.elapsed().as_secs_f64()
            );
        }
    }

    let transformed_header = build_transformed_header(header, &points, epsg)?;
    if let Some(path) = &cli.preprocessed_output {
        write_point_cloud(path, &transformed_header, &points, cli.force)
            .with_context(|| format!("写出预处理点云失败: {}", path.display()))?;
    }

    if !cli.quiet {
        eprintln!(
            "origin 预处理完成: {} 点，pivot=({:.3}, {:.3})，耗时 {:.2}s",
            points.len(),
            pivot.0,
            pivot.1,
            start.elapsed().as_secs_f64()
        );
    }

    Ok(PreparedPointCloud {
        header: transformed_header,
        points,
        summary: TransformSummary {
            source: origin_path.clone(),
            point_count: header.number_of_points(),
            epsg,
            origin_utm,
            pivot,
            yaw_deg: cli.yaw_deg,
            mapping: cli.mapping,
        },
    })
}

fn run_raster_only(cli: &Cli, reader: &mut Reader, header: &Header, start: Instant) -> Result<()> {
    let preview_path = cli
        .intensity_preview
        .as_ref()
        .context("--raster-only 模式下必须指定 --intensity-preview")?;
    let resolution = cli.intensity_resolution.unwrap_or(cli.voxel_size);
    let layout = build_raster_layout(header, resolution)?;
    let mut raster = vec![RasterCell::default(); layout.pixel_count];
    let mut input_points = 0_u64;

    if !cli.quiet {
        eprintln!(
            "开始生成强度图: input={}, path={}, resolution={}m",
            cli.input.display(),
            preview_path.display(),
            resolution
        );
    }

    for wrapped_point in reader.points() {
        let point = wrapped_point
            .with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
        input_points += 1;
        accumulate_raster_point(&mut raster, &layout, &point);

        if !cli.quiet && input_points % 5_000_000 == 0 {
            eprintln!(
                "已读取 {} 点，强度图栅格 {} x {}，耗时 {:.1}s",
                input_points,
                layout.width,
                layout.height,
                start.elapsed().as_secs_f64()
            );
        }
    }

    write_intensity_preview_from_raster(preview_path, &layout, &raster, header, cli.force)
        .with_context(|| format!("输出强度预览图失败: {}", preview_path.display()))?;

    if !cli.quiet {
        eprintln!(
            "完成: 输入 {} 点，输出强度图 {}，总耗时 {:.2}s",
            input_points,
            preview_path.display(),
            start.elapsed().as_secs_f64()
        );
    }

    Ok(())
}

fn run_raster_only_prepared(
    cli: &Cli,
    header: &Header,
    points: &[Point],
    start: Instant,
    summary: &TransformSummary,
) -> Result<()> {
    let preview_path = cli
        .intensity_preview
        .as_ref()
        .context("--raster-only 模式下必须指定 --intensity-preview")?;
    let resolution = cli.intensity_resolution.unwrap_or(cli.voxel_size);
    let layout = build_raster_layout(header, resolution)?;
    let mut raster = vec![RasterCell::default(); layout.pixel_count];

    if !cli.quiet {
        eprintln!(
            "开始生成强度图: input={}, path={}, resolution={}m（已应用 origin 预处理）",
            cli.input.display(),
            preview_path.display(),
            resolution
        );
        log_transform_summary(summary);
    }

    for point in points {
        accumulate_raster_point(&mut raster, &layout, point);
    }

    write_intensity_preview_from_raster(preview_path, &layout, &raster, header, cli.force)
        .with_context(|| format!("输出强度预览图失败: {}", preview_path.display()))?;

    if !cli.quiet {
        eprintln!(
            "完成: 输入 {} 点，输出强度图 {}，总耗时 {:.2}s",
            points.len(),
            preview_path.display(),
            start.elapsed().as_secs_f64()
        );
    }

    Ok(())
}

fn run_voxelize(cli: &Cli, reader: &mut Reader, header: &Header, start: Instant) -> Result<()> {
    let output_path = cli
        .output
        .as_ref()
        .context("非 --raster-only 模式下必须指定 --output")?;

    if !cli.quiet {
        eprintln!(
            "开始体素抽稀: input={}, output={}, voxel_size={}m, representative={:?}",
            cli.input.display(),
            output_path.display(),
            cli.voxel_size,
            cli.representative
        );
    }

    let reserve = cli.reserve.unwrap_or(0);
    let mut voxel_index: FxHashMap<VoxelKey, usize> =
        FxHashMap::with_capacity_and_hasher(reserve, Default::default());
    let mut selected: Vec<SelectedPoint> = Vec::with_capacity(reserve);

    let inv_voxel = 1.0 / cli.voxel_size;
    let mut input_points = 0_u64;

    for wrapped_point in reader.points() {
        let point = wrapped_point
            .with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
        input_points += 1;

        insert_selected_point(
            &mut voxel_index,
            &mut selected,
            point,
            inv_voxel,
            cli.voxel_size,
            cli.representative,
        )?;

        if !cli.quiet && input_points % 5_000_000 == 0 {
            eprintln!(
                "已读取 {} 点，当前保留 {} 点，耗时 {:.1}s",
                input_points,
                selected.len(),
                start.elapsed().as_secs_f64()
            );
        }
    }

    let selected_points: Vec<Point> = selected.into_iter().map(|item| item.point).collect();
    finalize_voxelize(cli, header, selected_points, input_points, output_path, start)
}

fn run_voxelize_prepared(
    cli: &Cli,
    header: &Header,
    points: &[Point],
    start: Instant,
    summary: &TransformSummary,
) -> Result<()> {
    let output_path = cli
        .output
        .as_ref()
        .context("非 --raster-only 模式下必须指定 --output")?;

    if !cli.quiet {
        eprintln!(
            "开始体素抽稀: input={}, output={}, voxel_size={}m, representative={:?}（已应用 origin 预处理）",
            cli.input.display(),
            output_path.display(),
            cli.voxel_size,
            cli.representative
        );
        log_transform_summary(summary);
    }

    let reserve = cli.reserve.unwrap_or(0);
    let mut voxel_index: FxHashMap<VoxelKey, usize> =
        FxHashMap::with_capacity_and_hasher(reserve, Default::default());
    let mut selected: Vec<SelectedPoint> = Vec::with_capacity(reserve);
    let inv_voxel = 1.0 / cli.voxel_size;

    for (idx, point) in points.iter().cloned().enumerate() {
        insert_selected_point(
            &mut voxel_index,
            &mut selected,
            point,
            inv_voxel,
            cli.voxel_size,
            cli.representative,
        )?;

        let input_points = idx as u64 + 1;
        if !cli.quiet && input_points % 5_000_000 == 0 {
            eprintln!(
                "已读取 {} 点，当前保留 {} 点，耗时 {:.1}s",
                input_points,
                selected.len(),
                start.elapsed().as_secs_f64()
            );
        }
    }

    let selected_points: Vec<Point> = selected.into_iter().map(|item| item.point).collect();
    finalize_voxelize(cli, header, selected_points, points.len() as u64, output_path, start)
}

fn run_voxelize_with_late_transform(
    cli: &Cli,
    reader: &mut Reader,
    header: &Header,
    start: Instant,
) -> Result<()> {
    let output_path = cli
        .output
        .as_ref()
        .context("非 --raster-only 模式下必须指定 --output")?;

    if !cli.quiet {
        eprintln!(
            "开始体素抽稀: input={}, output={}, voxel_size={}m, representative={:?}（origin 优化模式：抽稀后再变换保留点）",
            cli.input.display(),
            output_path.display(),
            cli.voxel_size,
            cli.representative
        );
    }

    let reserve = cli.reserve.unwrap_or(0);
    let mut voxel_index: FxHashMap<VoxelKey, usize> =
        FxHashMap::with_capacity_and_hasher(reserve, Default::default());
    let mut selected: Vec<SelectedPoint> = Vec::with_capacity(reserve);
    let mut pivot_acc = PivotAccumulator::default();
    let inv_voxel = 1.0 / cli.voxel_size;
    let mut input_points = 0_u64;

    for wrapped_point in reader.points() {
        let point = wrapped_point
            .with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
        input_points += 1;
        update_pivot_accumulator(&mut pivot_acc, &point);
        insert_selected_point(
            &mut voxel_index,
            &mut selected,
            point,
            inv_voxel,
            cli.voxel_size,
            cli.representative,
        )?;

        if !cli.quiet && input_points % 5_000_000 == 0 {
            eprintln!(
                "已读取 {} 点，当前保留 {} 点，耗时 {:.1}s",
                input_points,
                selected.len(),
                start.elapsed().as_secs_f64()
            );
        }
    }

    let pivot = pivot_from_accumulator(&pivot_acc, cli.pivot);
    let config = build_transform_config(cli, pivot)?;
    let summary = TransformSummary {
        source: cli
            .origin
            .as_ref()
            .cloned()
            .context("启用预处理时必须提供 --origin")?,
        point_count: input_points,
        epsg: config.epsg,
        origin_utm: config.origin_utm,
        pivot,
        yaw_deg: cli.yaw_deg,
        mapping: cli.mapping,
    };

    if !cli.quiet {
        eprintln!(
            "开始 origin 后置变换: 保留点 {}，pivot=({:.3}, {:.3})，epsg=EPSG:{}",
            selected.len(),
            pivot.0,
            pivot.1,
            config.epsg
        );
        log_transform_summary(&summary);
    }

    let mut selected_points: Vec<Point> = selected.into_iter().map(|item| item.point).collect();
    selected_points
        .par_iter_mut()
        .try_for_each(|point| apply_transform(point, &config))?;
    let transformed_header = build_transformed_header(header, &selected_points, config.epsg)?;

    finalize_voxelize(
        cli,
        &transformed_header,
        selected_points,
        input_points,
        output_path,
        start,
    )
}

fn finalize_voxelize(
    cli: &Cli,
    header: &Header,
    selected_points: Vec<Point>,
    input_points: u64,
    output_path: &Path,
    start: Instant,
) -> Result<()> {
    if let Some(preview_path) = &cli.intensity_preview {
        let resolution = cli.intensity_resolution.unwrap_or(cli.voxel_size);
        if !cli.quiet {
            eprintln!(
                "开始输出强度预览图: path={}, resolution={}m",
                preview_path.display(),
                resolution
            );
        }
        write_intensity_preview_points(preview_path, resolution, &selected_points, header, cli.force)
            .with_context(|| format!("输出强度预览图失败: {}", preview_path.display()))?;
    }

    let unique_points = selected_points.len() as u64;
    let output_header = rebuild_header_for_points(header, &selected_points)?;
    write_point_cloud(output_path, &output_header, &selected_points, cli.force)
        .with_context(|| format!("无法创建输出文件: {}", output_path.display()))?;

    if !cli.quiet {
        let elapsed = start.elapsed().as_secs_f64();
        let ratio = if input_points == 0 {
            0.0
        } else {
            unique_points as f64 / input_points as f64
        };
        eprintln!(
            "完成: 输入 {} 点 -> 输出 {} 点，保留率 {:.2}%，总耗时 {:.2}s",
            input_points,
            unique_points,
            ratio * 100.0,
            elapsed
        );
    }

    Ok(())
}

fn validate_args(cli: &Cli) -> Result<()> {
    if !(cli.voxel_size.is_finite() && cli.voxel_size > 0.0) {
        bail!("--voxel-size 必须是正数");
    }
    if !cli.yaw_deg.is_finite() {
        bail!("--yaw-deg 必须是有限数值");
    }

    if cli.raster_only {
        if cli.output.is_some() {
            bail!("--raster-only 模式下不要传 --output");
        }
        if cli.intensity_preview.is_none() {
            bail!("--raster-only 模式下必须指定 --intensity-preview");
        }
    } else if cli.output.is_none() {
        bail!("非 --raster-only 模式下必须指定 --output");
    }

    if cli.output.is_none() && cli.intensity_preview.is_none() {
        bail!("至少指定 --output 或 --intensity-preview");
    }

    if let Some(output) = &cli.output {
        validate_output_path(output, cli.force, "输出文件")?;
        if cli.input == *output {
            bail!("输入文件和输出文件不能相同");
        }
    }

    if let Some(threads) = cli.threads
        && threads == 0
    {
        bail!("--threads 必须大于 0");
    }

    if let Some(resolution) = cli.intensity_resolution
        && !(resolution.is_finite() && resolution > 0.0)
    {
        bail!("--intensity-resolution 必须是正数");
    }

    if let Some(origin) = &cli.origin {
        if !origin.exists() {
            bail!("origin 文件不存在: {}", origin.display());
        }
    }

    if let Some(epsg) = cli.epsg {
        epsg_to_utm_zone(epsg)?;
    }

    if let Some(path) = &cli.preprocessed_output {
        if cli.origin.is_none() {
            bail!("--preprocessed-output 必须和 --origin 一起使用");
        }
        validate_output_path(path, cli.force, "预处理点云")?;
        if *path == cli.input {
            bail!("--preprocessed-output 不能和 --input 相同");
        }
        if let Some(output) = &cli.output
            && *path == *output
        {
            bail!("--preprocessed-output 不能和 --output 相同");
        }
    }

    if let Some(path) = &cli.intensity_preview {
        preview_format(path)?;
        validate_output_path(path, cli.force, "强度预览图")?;

        let world_file = world_file_path(path)?;
        validate_output_path(&world_file, cli.force, "世界文件")?;

        let aux_xml = aux_xml_path(path);
        validate_output_path(&aux_xml, cli.force, "Aux XML 文件")?;

        let vrt_file = vrt_path(path);
        validate_output_path(&vrt_file, cli.force, "VRT 文件")?;

        let prj_file = sidecar_path(path, "prj");
        validate_output_path(&prj_file, cli.force, "PRJ 文件")?;
    }

    Ok(())
}

fn validate_output_path(path: &Path, force: bool, label: &str) -> Result<()> {
    if path.exists() && !force {
        bail!("{}已存在: {}，如需覆盖请加 --force", label, path.display());
    }
    Ok(())
}

fn configure_threads(threads: Option<usize>) -> Result<()> {
    if let Some(threads) = threads {
        ThreadPoolBuilder::new()
            .num_threads(threads)
            .build_global()
            .context("初始化 rayon 线程池失败")?;
    }
    Ok(())
}

fn read_origin(path: &Path) -> Result<OriginInfo> {
    let content = fs::read_to_string(path)
        .with_context(|| format!("无法读取 origin 文件: {}", path.display()))?;
    let parts: Vec<_> = content.split_whitespace().collect();
    if parts.len() < 3 {
        bail!("origin 文件至少需要包含 3 个值: lat lon alt");
    }
    Ok(OriginInfo {
        lat: parts[0].parse().context("origin 中的纬度解析失败")?,
        lon: parts[1].parse().context("origin 中的经度解析失败")?,
        alt: parts[2].parse().context("origin 中的高程解析失败")?,
    })
}

fn infer_utm_epsg(lat: f64, lon: f64) -> u16 {
    let zone = ((lon + 180.0) / 6.0).floor() as u16 + 1;
    if lat >= 0.0 {
        32600 + zone
    } else {
        32700 + zone
    }
}

fn get_pivot(points: &[Point], mode: PivotMode) -> (f64, f64) {
    if points.is_empty() || mode == PivotMode::Zero {
        return (0.0, 0.0);
    }

    match mode {
        PivotMode::Centroid => {
            let (sum_x, sum_y) = points.iter().fold((0.0, 0.0), |(sx, sy), point| {
                (sx + point.x, sy + point.y)
            });
            let count = points.len() as f64;
            (sum_x / count, sum_y / count)
        }
        PivotMode::BboxCenter => {
            let mut min_x = f64::INFINITY;
            let mut min_y = f64::INFINITY;
            let mut max_x = f64::NEG_INFINITY;
            let mut max_y = f64::NEG_INFINITY;
            for point in points {
                min_x = min_x.min(point.x);
                min_y = min_y.min(point.y);
                max_x = max_x.max(point.x);
                max_y = max_y.max(point.y);
            }
            ((min_x + max_x) * 0.5, (min_y + max_y) * 0.5)
        }
        PivotMode::Zero => (0.0, 0.0),
    }
}

fn apply_transform(point: &mut Point, config: &TransformConfig) -> Result<()> {
    let (x_rot, y_rot) = rotate_xy(
        point.x,
        point.y,
        config.pivot.0,
        config.pivot.1,
        config.yaw_deg,
    );

    match config.mapping {
        MappingMode::Flat => {
            point.x = config.origin_utm.0 + x_rot;
            point.y = config.origin_utm.1 + y_rot;
            point.z = config.origin_utm.2 + point.z;
        }
        MappingMode::Enu => {
            let (x_ecef, y_ecef, z_ecef) = enu_to_ecef(
                x_rot,
                y_rot,
                point.z,
                config.origin.lat,
                config.origin.lon,
                config.origin_ecef,
            );
            let (lat, lon, alt) = ecef_to_geodetic(x_ecef, y_ecef, z_ecef);
            let (x_utm, y_utm, z_utm) = utm_from_geodetic(lat, lon, alt, config.epsg)?;
            point.x = x_utm;
            point.y = y_utm;
            point.z = z_utm;
        }
    }

    Ok(())
}

fn rotate_xy(x: f64, y: f64, pivot_x: f64, pivot_y: f64, yaw_deg: f64) -> (f64, f64) {
    let theta = (-yaw_deg).to_radians();
    let cos_t = theta.cos();
    let sin_t = theta.sin();
    let dx = x - pivot_x;
    let dy = y - pivot_y;
    (
        dx * cos_t - dy * sin_t + pivot_x,
        dx * sin_t + dy * cos_t + pivot_y,
    )
}

fn ecef_from_geodetic(lat_deg: f64, lon_deg: f64, alt: f64) -> (f64, f64, f64) {
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let sin_lon = lon.sin();
    let cos_lon = lon.cos();
    let e2 = wgs84_e2();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let x = (n + alt) * cos_lat * cos_lon;
    let y = (n + alt) * cos_lat * sin_lon;
    let z = (n * (1.0 - e2) + alt) * sin_lat;
    (x, y, z)
}

fn enu_to_ecef(
    east: f64,
    north: f64,
    up: f64,
    lat_deg: f64,
    lon_deg: f64,
    origin_ecef: (f64, f64, f64),
) -> (f64, f64, f64) {
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let sin_lon = lon.sin();
    let cos_lon = lon.cos();
    let (x0, y0, z0) = origin_ecef;

    (
        x0 - sin_lon * east - sin_lat * cos_lon * north + cos_lat * cos_lon * up,
        y0 + cos_lon * east - sin_lat * sin_lon * north + cos_lat * sin_lon * up,
        z0 + cos_lat * north + sin_lat * up,
    )
}

fn ecef_to_geodetic(x: f64, y: f64, z: f64) -> (f64, f64, f64) {
    let e2 = wgs84_e2();
    let b = WGS84_A * (1.0 - WGS84_F);
    let ep2 = (WGS84_A * WGS84_A - b * b) / (b * b);
    let p = x.hypot(y);
    let lon = y.atan2(x);
    let theta = (z * WGS84_A).atan2(p * b);
    let sin_theta = theta.sin();
    let cos_theta = theta.cos();
    let lat = (z + ep2 * b * sin_theta.powi(3)).atan2(p - e2 * WGS84_A * cos_theta.powi(3));
    let sin_lat = lat.sin();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let alt = p / lat.cos() - n;
    (lat.to_degrees(), lon.to_degrees(), alt)
}

fn utm_from_geodetic(lat_deg: f64, lon_deg: f64, alt: f64, epsg: u16) -> Result<(f64, f64, f64)> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let lat = lat_deg.to_radians();
    let lon = lon_deg.to_radians();
    let lon_origin = ((f64::from(zone) - 1.0) * 6.0 - 180.0 + 3.0).to_radians();
    let e2 = wgs84_e2();
    let ep2 = e2 / (1.0 - e2);
    let sin_lat = lat.sin();
    let cos_lat = lat.cos();
    let tan_lat = lat.tan();
    let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
    let t = tan_lat * tan_lat;
    let c = ep2 * cos_lat * cos_lat;
    let a = cos_lat * (lon - lon_origin);

    let m = WGS84_A
        * ((1.0 - e2 / 4.0 - 3.0 * e2.powi(2) / 64.0 - 5.0 * e2.powi(3) / 256.0) * lat
            - (3.0 * e2 / 8.0 + 3.0 * e2.powi(2) / 32.0 + 45.0 * e2.powi(3) / 1024.0)
                * (2.0 * lat).sin()
            + (15.0 * e2.powi(2) / 256.0 + 45.0 * e2.powi(3) / 1024.0) * (4.0 * lat).sin()
            - (35.0 * e2.powi(3) / 3072.0) * (6.0 * lat).sin());

    let easting = UTM_FALSE_EASTING
        + UTM_K0
            * n
            * (a
                + (1.0 - t + c) * a.powi(3) / 6.0
                + (5.0 - 18.0 * t + t * t + 72.0 * c - 58.0 * ep2) * a.powi(5) / 120.0);

    let mut northing = UTM_K0
        * (m
            + n
                * tan_lat
                * (a.powi(2) / 2.0
                    + (5.0 - t + 9.0 * c + 4.0 * c * c) * a.powi(4) / 24.0
                    + (61.0 - 58.0 * t + t * t + 600.0 * c - 330.0 * ep2) * a.powi(6) / 720.0));
    if !northern {
        northing += UTM_FALSE_NORTHING_SOUTH;
    }

    Ok((easting, northing, alt))
}

fn epsg_to_utm_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("目前仅支持 WGS84 UTM EPSG:32601-32660 或 EPSG:32701-32760"),
    }
}

fn build_transformed_header(original_header: &Header, points: &[Point], epsg: u16) -> Result<Header> {
    let mut builder = Builder::from(original_header.clone());
    builder.version = original_header.version();
    builder.vlrs.retain(|vlr| !vlr.is_crs());

    if let Some((min_x, min_y, min_z)) = min_coordinates(points) {
        builder.transforms.x.offset = min_x.floor();
        builder.transforms.y.offset = min_y.floor();
        builder.transforms.z.offset = min_z.floor();
    }

    builder.vlrs.extend(build_geotiff_crs_vlrs(epsg)?);

    let mut header = builder.into_header().context("重建带 CRS 的 LAS header 失败")?;
    header.clear();
    for point in points {
        header.add_point(point);
    }
    Ok(header)
}

fn build_geotiff_crs_vlrs(epsg: u16) -> Result<Vec<Vlr>> {
    let citation = build_utm_citation(epsg)?;
    let ascii_bytes = citation.as_bytes();
    let ascii_len: u16 = ascii_bytes
        .len()
        .try_into()
        .context("GeoTIFF citation 过长")?;

    let mut geokey_data = Vec::with_capacity(32);
    for value in [
        1_u16, 1, 0, 3, // GeoKeyDirectory header
        1024, 0, 1, 1, // GTModelTypeGeoKey = Projected
        3072, 0, 1, epsg, // ProjectedCSTypeGeoKey = EPSG code
        3073, 34737, ascii_len, 0, // PCSCitationGeoKey -> GeoAsciiParams
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

fn rebuild_header_for_points(template: &Header, points: &[Point]) -> Result<Header> {
    let mut builder = Builder::from(template.clone());

    if let Some((min_x, min_y, min_z)) = min_coordinates(points) {
        builder.transforms.x.offset = min_x.floor();
        builder.transforms.y.offset = min_y.floor();
        builder.transforms.z.offset = min_z.floor();
    }

    let mut header = builder.into_header().context("重建 LAS header 失败")?;
    header.clear();
    for point in points {
        header.add_point(point);
    }
    Ok(header)
}

fn min_coordinates(points: &[Point]) -> Option<(f64, f64, f64)> {
    if points.is_empty() {
        return None;
    }

    let mut min_x = f64::INFINITY;
    let mut min_y = f64::INFINITY;
    let mut min_z = f64::INFINITY;
    for point in points {
        min_x = min_x.min(point.x);
        min_y = min_y.min(point.y);
        min_z = min_z.min(point.z);
    }
    Some((min_x, min_y, min_z))
}

fn build_utm_wkt(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let central_meridian = (i32::from(zone) - 1) * 6 - 180 + 3;
    let false_northing = if northern { 0.0 } else { UTM_FALSE_NORTHING_SOUTH };
    let hemisphere = if northern { "N" } else { "S" };
    Ok(format!(
        "PROJCS[\"WGS 84 / UTM zone {zone}{hemisphere}\",GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563]],PRIMEM[\"Greenwich\",0],UNIT[\"degree\",0.0174532925199433]],PROJECTION[\"Transverse_Mercator\"],PARAMETER[\"latitude_of_origin\",0],PARAMETER[\"central_meridian\",{central_meridian}],PARAMETER[\"scale_factor\",0.9996],PARAMETER[\"false_easting\",500000],PARAMETER[\"false_northing\",{false_northing}],UNIT[\"metre\",1]]"
    ))
}

fn build_utm_citation(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    Ok(format!(
        "WGS 84 / UTM zone {}{}",
        zone,
        if northern { "N" } else { "S" }
    ))
}

impl Default for PivotAccumulator {
    fn default() -> Self {
        Self {
            count: 0,
            sum_x: 0.0,
            sum_y: 0.0,
            min_x: f64::INFINITY,
            min_y: f64::INFINITY,
            max_x: f64::NEG_INFINITY,
            max_y: f64::NEG_INFINITY,
        }
    }
}

fn update_pivot_accumulator(acc: &mut PivotAccumulator, point: &Point) {
    acc.count += 1;
    acc.sum_x += point.x;
    acc.sum_y += point.y;
    acc.min_x = acc.min_x.min(point.x);
    acc.min_y = acc.min_y.min(point.y);
    acc.max_x = acc.max_x.max(point.x);
    acc.max_y = acc.max_y.max(point.y);
}

fn pivot_from_accumulator(acc: &PivotAccumulator, mode: PivotMode) -> (f64, f64) {
    if acc.count == 0 || mode == PivotMode::Zero {
        return (0.0, 0.0);
    }

    match mode {
        PivotMode::Centroid => (acc.sum_x / acc.count as f64, acc.sum_y / acc.count as f64),
        PivotMode::BboxCenter => ((acc.min_x + acc.max_x) * 0.5, (acc.min_y + acc.max_y) * 0.5),
        PivotMode::Zero => (0.0, 0.0),
    }
}

fn build_transform_config(cli: &Cli, pivot: (f64, f64)) -> Result<TransformConfig> {
    let origin_path = cli
        .origin
        .as_ref()
        .context("启用预处理时必须提供 --origin")?;
    let origin = read_origin(origin_path)?;
    let epsg = cli.epsg.unwrap_or_else(|| infer_utm_epsg(origin.lat, origin.lon));
    let origin_utm = utm_from_geodetic(origin.lat, origin.lon, origin.alt, epsg)?;
    let origin_ecef = ecef_from_geodetic(origin.lat, origin.lon, origin.alt);
    Ok(TransformConfig {
        origin,
        epsg,
        origin_utm,
        origin_ecef,
        pivot,
        yaw_deg: cli.yaw_deg,
        mapping: cli.mapping,
    })
}

fn write_point_cloud(path: &Path, header: &Header, points: &[Point], _force: bool) -> Result<()> {
    ensure_parent_dir(path)?;
    let output_header = sanitize_header_for_write(header)?;
    let mut writer = Writer::from_path(path, output_header)
        .with_context(|| format!("无法创建输出文件: {}", path.display()))?;
    for point in points {
        writer
            .write_point(point.clone())
            .with_context(|| format!("写出点记录失败: {}", path.display()))?;
    }
    Ok(())
}

fn ensure_parent_dir(path: &Path) -> Result<()> {
    if let Some(parent) = path.parent()
        && !parent.as_os_str().is_empty()
    {
        fs::create_dir_all(parent)
            .with_context(|| format!("无法创建目录: {}", parent.display()))?;
    }
    Ok(())
}

fn sanitize_header_for_write(header: &Header) -> Result<Header> {
    let mut builder = Builder::from(header.clone());
    builder.vlrs.retain(|vlr| !laz::is_laszip_vlr(vlr));
    builder.into_header().context("清理写出 header 失败")
}

fn log_transform_summary(summary: &TransformSummary) {
    eprintln!(
        "预处理摘要: origin={}, points={}, epsg=EPSG:{}, utm_origin=({:.3}, {:.3}, {:.3}), pivot=({:.3}, {:.3}), yaw={}deg, mapping={:?}",
        summary.source.display(),
        summary.point_count,
        summary.epsg,
        summary.origin_utm.0,
        summary.origin_utm.1,
        summary.origin_utm.2,
        summary.pivot.0,
        summary.pivot.1,
        summary.yaw_deg,
        summary.mapping
    );
}

fn insert_selected_point(
    voxel_index: &mut FxHashMap<VoxelKey, usize>,
    selected: &mut Vec<SelectedPoint>,
    point: Point,
    inv_voxel: f64,
    voxel_size: f64,
    representative: RepresentativeMode,
) -> Result<()> {
    let key = voxel_key(&point, inv_voxel)
        .with_context(|| format!("点坐标超出可支持范围: ({}, {}, {})", point.x, point.y, point.z))?;
    let score = score_point(&point, key, voxel_size, representative);

    match voxel_index.entry(key) {
        Entry::Occupied(entry) => {
            if representative == RepresentativeMode::Center {
                let idx = *entry.get();
                if score < selected[idx].score {
                    selected[idx] = SelectedPoint { point, score };
                }
            }
        }
        Entry::Vacant(entry) => {
            let idx = selected.len();
            entry.insert(idx);
            selected.push(SelectedPoint { point, score });
        }
    }

    Ok(())
}

fn voxel_key(point: &Point, inv_voxel: f64) -> Result<VoxelKey> {
    Ok(VoxelKey {
        x: quantize(point.x, inv_voxel)?,
        y: quantize(point.y, inv_voxel)?,
        z: quantize(point.z, inv_voxel)?,
    })
}

fn quantize(value: f64, inv_voxel: f64) -> Result<i32> {
    let index = (value * inv_voxel).floor();
    if !(i32::MIN as f64..=i32::MAX as f64).contains(&index) {
        bail!("量化后的体素索引超出 i32 范围: {}", index);
    }
    Ok(index as i32)
}

fn score_point(point: &Point, key: VoxelKey, voxel_size: f64, mode: RepresentativeMode) -> f64 {
    match mode {
        RepresentativeMode::First => 0.0,
        RepresentativeMode::Center => {
            let cx = (f64::from(key.x) + 0.5) * voxel_size;
            let cy = (f64::from(key.y) + 0.5) * voxel_size;
            let cz = (f64::from(key.z) + 0.5) * voxel_size;
            let dx = point.x - cx;
            let dy = point.y - cy;
            let dz = point.z - cz;
            dx * dx + dy * dy + dz * dz
        }
    }
}

fn write_intensity_preview_points(
    preview_path: &PathBuf,
    resolution: f64,
    points: &[Point],
    header: &Header,
    force: bool,
) -> Result<()> {
    let layout = build_raster_layout(header, resolution)?;
    let mut raster = vec![RasterCell::default(); layout.pixel_count];

    for point in points {
        accumulate_raster_point(&mut raster, &layout, point);
    }

    write_intensity_preview_from_raster(preview_path, &layout, &raster, header, force)
}

fn build_raster_layout(header: &Header, resolution: f64) -> Result<RasterLayout> {
    let bounds = header.bounds();
    let min_x = bounds.min.x;
    let min_y = bounds.min.y;
    let max_x = bounds.max.x;
    let max_y = bounds.max.y;

    let width = ((max_x - min_x) / resolution).ceil().max(1.0) as usize;
    let height = ((max_y - min_y) / resolution).ceil().max(1.0) as usize;
    let pixel_count = width
        .checked_mul(height)
        .context("栅格尺寸过大，像素数量溢出")?;

    if width > 100_000 || height > 100_000 || pixel_count > 400_000_000 {
        bail!(
            "栅格过大: {} x {}，请增大 --intensity-resolution 后重试",
            width,
            height
        );
    }

    Ok(RasterLayout {
        min_x,
        min_y,
        max_x,
        max_y,
        resolution,
        width,
        height,
        pixel_count,
    })
}

fn accumulate_raster_point(raster: &mut [RasterCell], layout: &RasterLayout, point: &Point) {
    let x = point.x;
    let y = point.y;
    if x < layout.min_x || x > layout.max_x || y < layout.min_y || y > layout.max_y {
        return;
    }

    let col =
        (((x - layout.min_x) / layout.resolution).floor() as isize).clamp(0, layout.width as isize - 1);
    let row =
        (((layout.max_y - y) / layout.resolution).floor() as isize).clamp(0, layout.height as isize - 1);
    let idx = row as usize * layout.width + col as usize;
    let cell = &mut raster[idx];
    cell.intensity_sum += f64::from(point.intensity);
    cell.count += 1;
}

fn write_intensity_preview_from_raster(
    preview_path: &PathBuf,
    layout: &RasterLayout,
    raster: &[RasterCell],
    header: &Header,
    force: bool,
) -> Result<()> {
    let mut values = Vec::new();
    values.reserve(raster.len());
    for cell in raster {
        if cell.count > 0 {
            values.push(cell.intensity_sum / f64::from(cell.count));
        }
    }

    if values.is_empty() {
        bail!("没有可用于生成强度图的有效像素");
    }

    values.sort_by(|a, b| a.total_cmp(b));
    let last = values.len() - 1;
    let low = values[((last as f64) * 0.02).round() as usize];
    let high = values[((last as f64) * 0.98).round() as usize].max(low + 1.0);

    let mut image = ImageBuffer::from_pixel(
        layout.width as u32,
        layout.height as u32,
        Rgba([0, 0, 0, 0]),
    );
    for (idx, cell) in raster.iter().enumerate() {
        if cell.count == 0 {
            continue;
        }

        let value = cell.intensity_sum / f64::from(cell.count);
        let gray = normalize_to_u8(value, low, high);
        let x = (idx % layout.width) as u32;
        let y = (idx / layout.width) as u32;
        image.put_pixel(x, y, Rgba([gray, gray, gray, 255]));
    }

    ensure_parent_dir(preview_path)?;
    image
        .save(preview_path)
        .with_context(|| format!("无法写出强度预览图: {}", preview_path.display()))?;
    write_world_file(
        preview_path,
        layout.min_x,
        layout.max_y,
        layout.resolution,
        force,
    )?;
    write_prj_file(preview_path, header, force)?;
    write_aux_xml_file(preview_path, layout, header, force)?;
    write_vrt_file(preview_path, layout, header, force)?;
    Ok(())
}

fn normalize_to_u8(value: f64, low: f64, high: f64) -> u8 {
    if value <= low {
        return 0;
    }
    if value >= high {
        return 255;
    }
    (((value - low) / (high - low)) * 255.0).round() as u8
}

fn write_world_file(
    preview_path: &PathBuf,
    min_x: f64,
    max_y: f64,
    resolution: f64,
    force: bool,
) -> Result<()> {
    let world_file = world_file_path(preview_path)?;
    if world_file.exists() && !force {
        bail!(
            "世界文件已存在: {}，如需覆盖请加 --force",
            world_file.display()
        );
    }

    let content = format!(
        "{:.12}\n0.0\n0.0\n-{:.12}\n{:.12}\n{:.12}\n",
        resolution,
        resolution,
        min_x + resolution * 0.5,
        max_y - resolution * 0.5
    );
    fs::write(&world_file, content)
        .with_context(|| format!("无法写出世界文件: {}", world_file.display()))?;
    Ok(())
}

fn write_prj_file(preview_path: &PathBuf, header: &Header, force: bool) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };

    let prj_file = sidecar_path(preview_path, "prj");
    if prj_file.exists() && !force {
        bail!(
            "PRJ 文件已存在: {}，如需覆盖请加 --force",
            prj_file.display()
        );
    }

    fs::write(&prj_file, wkt)
        .with_context(|| format!("无法写出 PRJ 文件: {}", prj_file.display()))?;
    Ok(())
}

fn write_aux_xml_file(preview_path: &PathBuf, layout: &RasterLayout, header: &Header, force: bool) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };

    let aux_xml = aux_xml_path(preview_path);
    if aux_xml.exists() && !force {
        bail!(
            "Aux XML 文件已存在: {}，如需覆盖请加 --force",
            aux_xml.display()
        );
    }

    let content = format!(
        "<PAMDataset>\n  <SRS>{}</SRS>\n  <GeoTransform>{:.12}, {:.12}, 0.0, {:.12}, 0.0, -{:.12}</GeoTransform>\n</PAMDataset>\n",
        xml_escape(&wkt),
        layout.min_x,
        layout.resolution,
        layout.max_y,
        layout.resolution
    );
    fs::write(&aux_xml, content)
        .with_context(|| format!("无法写出 Aux XML 文件: {}", aux_xml.display()))?;
    Ok(())
}

fn write_vrt_file(preview_path: &PathBuf, layout: &RasterLayout, header: &Header, force: bool) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };

    let vrt_path = vrt_path(preview_path);
    if vrt_path.exists() && !force {
        bail!(
            "VRT 文件已存在: {}，如需覆盖请加 --force",
            vrt_path.display()
        );
    }

    let source_name = preview_path
        .file_name()
        .and_then(|name| name.to_str())
        .context("无法获取 PNG 文件名以生成 VRT")?;
    let geotransform = format!(
        "{:.12}, {:.12}, 0.0, {:.12}, 0.0, -{:.12}",
        layout.min_x,
        layout.resolution,
        layout.max_y,
        layout.resolution
    );
    let mut xml = String::new();
    xml.push_str(&format!(
        "<VRTDataset rasterXSize=\"{}\" rasterYSize=\"{}\">\n",
        layout.width, layout.height
    ));
    xml.push_str(&format!("  <SRS>{}</SRS>\n", xml_escape(&wkt)));
    xml.push_str(&format!("  <GeoTransform>{}</GeoTransform>\n", geotransform));

    for band in 1..=4 {
        xml.push_str(&format!("  <VRTRasterBand dataType=\"Byte\" band=\"{}\">\n", band));
        if band == 4 {
            xml.push_str("    <ColorInterp>Alpha</ColorInterp>\n");
        } else {
            let interp = match band {
                1 => "Red",
                2 => "Green",
                _ => "Blue",
            };
            xml.push_str(&format!("    <ColorInterp>{}</ColorInterp>\n", interp));
        }
        xml.push_str("    <SimpleSource>\n");
        xml.push_str(&format!(
            "      <SourceFilename relativeToVRT=\"1\">{}</SourceFilename>\n",
            xml_escape(source_name)
        ));
        xml.push_str(&format!("      <SourceBand>{}</SourceBand>\n", band));
        xml.push_str(&format!(
            "      <SourceProperties RasterXSize=\"{}\" RasterYSize=\"{}\" DataType=\"Byte\" BlockXSize=\"{}\" BlockYSize=\"1\" />\n",
            layout.width,
            layout.height,
            layout.width
        ));
        xml.push_str(&format!(
            "      <SrcRect xOff=\"0\" yOff=\"0\" xSize=\"{}\" ySize=\"{}\" />\n",
            layout.width, layout.height
        ));
        xml.push_str(&format!(
            "      <DstRect xOff=\"0\" yOff=\"0\" xSize=\"{}\" ySize=\"{}\" />\n",
            layout.width, layout.height
        ));
        xml.push_str("    </SimpleSource>\n");
        xml.push_str("  </VRTRasterBand>\n");
    }
    xml.push_str("</VRTDataset>\n");

    fs::write(&vrt_path, xml).with_context(|| format!("无法写出 VRT 文件: {}", vrt_path.display()))?;
    Ok(())
}

fn header_crs_wkt(header: &Header) -> Result<Option<String>> {
    if let Some(wkt_bytes) = header.get_wkt_crs_bytes() {
        return Ok(Some(String::from_utf8_lossy(wkt_bytes).into_owned()));
    }

    let Some(epsg) = extract_projected_epsg(header)? else {
        return Ok(None);
    };
    Ok(Some(build_utm_wkt(epsg)?))
}

fn extract_projected_epsg(header: &Header) -> Result<Option<u16>> {
    let Some(geotiff) = header.get_geotiff_crs()? else {
        return Ok(None);
    };

    for entry in geotiff.entries {
        if entry.id == 3072
            && let GeoTiffData::U16(code) = entry.data
        {
            return Ok(Some(code));
        }
    }

    Ok(None)
}

fn sidecar_path(path: &Path, extension: &str) -> PathBuf {
    path.with_extension(extension)
}

fn preview_format(path: &Path) -> Result<&'static str> {
    let ext = path
        .extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| ext.to_ascii_lowercase())
        .context("--intensity-preview 需要带文件扩展名")?;

    match ext.as_str() {
        "png" => Ok("png"),
        _ => bail!("--intensity-preview 目前仅支持输出 .png"),
    }
}

fn world_file_path(path: &Path) -> Result<PathBuf> {
    preview_format(path)?;
    Ok(sidecar_path(path, "pgw"))
}

fn aux_xml_path(path: &Path) -> PathBuf {
    PathBuf::from(format!("{}.aux.xml", path.display()))
}

fn vrt_path(path: &Path) -> PathBuf {
    PathBuf::from(format!("{}.vrt", path.display()))
}

fn xml_escape(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('\"', "&quot;")
        .replace('\'', "&apos;")
}

fn wgs84_e2() -> f64 {
    WGS84_F * (2.0 - WGS84_F)
}

#[allow(dead_code)]
fn rad_to_deg(rad: f64) -> f64 {
    rad * 180.0 / PI
}
