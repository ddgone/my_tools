use anyhow::{bail, Context, Result};
use clap::{Parser, ValueEnum};
use image::{ImageBuffer, Rgba};
use las::{Header, Point, Reader, Writer};
use rayon::ThreadPoolBuilder;
use rustc_hash::FxHashMap;
use std::collections::hash_map::Entry;
use std::fs;
use std::path::PathBuf;
use std::time::Instant;

#[derive(Debug, Parser)]
#[command(
    author,
    version,
    about = "对 LAS/LAZ 进行体素化抽稀，或仅输出带地理定位的强度 PNG"
)]
struct Cli {
    #[arg(short, long, value_name = "FILE")]
    input: PathBuf,

    #[arg(short, long, value_name = "FILE")]
    output: Option<PathBuf>,

    #[arg(long, default_value_t = 0.1)]
    voxel_size: f64,

    #[arg(long, value_enum, default_value_t = RepresentativeMode::Center)]
    representative: RepresentativeMode,

    #[arg(long, value_name = "COUNT")]
    reserve: Option<usize>,

    #[arg(long, value_name = "N")]
    threads: Option<usize>,

    #[arg(long, value_name = "PNG")]
    intensity_preview: Option<PathBuf>,

    #[arg(long, value_name = "METERS")]
    intensity_resolution: Option<f64>,

    #[arg(long)]
    raster_only: bool,

    #[arg(long)]
    force: bool,

    #[arg(long)]
    quiet: bool,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
enum RepresentativeMode {
    First,
    Center,
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

fn main() -> Result<()> {
    let cli = Cli::parse();
    validate_args(&cli)?;
    configure_threads(cli.threads)?;

    let start = Instant::now();
    let mut reader = Reader::from_path(&cli.input)
        .with_context(|| format!("无法打开输入文件: {}", cli.input.display()))?;
    let header = reader.header().clone();

    if cli.raster_only {
        run_raster_only(&cli, &mut reader, &header, start)?;
    } else {
        run_voxelize(&cli, &mut reader, &header, start)?;
    }

    Ok(())
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
        let point =
            wrapped_point.with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
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
        let point =
            wrapped_point.with_context(|| format!("读取点记录失败: {}", cli.input.display()))?;
        input_points += 1;

        let key = voxel_key(&point, inv_voxel).with_context(|| {
            format!(
                "点坐标超出可支持范围: ({}, {}, {})",
                point.x, point.y, point.z
            )
        })?;
        let score = score_point(&point, key, cli.voxel_size, cli.representative);

        match voxel_index.entry(key) {
            Entry::Occupied(entry) => {
                if cli.representative == RepresentativeMode::Center {
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

        if !cli.quiet && input_points % 5_000_000 == 0 {
            eprintln!(
                "已读取 {} 点，当前保留 {} 点，耗时 {:.1}s",
                input_points,
                selected.len(),
                start.elapsed().as_secs_f64()
            );
        }
    }

    if let Some(preview_path) = &cli.intensity_preview {
        let resolution = cli.intensity_resolution.unwrap_or(cli.voxel_size);
        if !cli.quiet {
            eprintln!(
                "开始输出强度预览图: path={}, resolution={}m",
                preview_path.display(),
                resolution
            );
        }
        write_intensity_preview(preview_path, resolution, &selected, header, cli.force)
            .with_context(|| format!("输出强度预览图失败: {}", preview_path.display()))?;
    }

    let unique_points = selected.len() as u64;
    let mut writer = Writer::from_path(output_path, header.clone())
        .with_context(|| format!("无法创建输出文件: {}", output_path.display()))?;

    for item in selected {
        writer
            .write_point(item.point)
            .with_context(|| format!("写出点记录失败: {}", output_path.display()))?;
    }

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
        if output.exists() && !cli.force {
            bail!("输出文件已存在: {}，如需覆盖请加 --force", output.display());
        }

        if cli.input == *output {
            bail!("输入文件和输出文件不能相同");
        }
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

    if let Some(path) = &cli.intensity_preview {
        if path
            .extension()
            .and_then(|ext| ext.to_str())
            .map(|ext| !ext.eq_ignore_ascii_case("png"))
            .unwrap_or(true)
        {
            bail!("--intensity-preview 目前仅支持输出 .png");
        }

        if path.exists() && !cli.force {
            bail!("强度预览图已存在: {}，如需覆盖请加 --force", path.display());
        }

        let world_file = sidecar_path(path, "pgw");
        if world_file.exists() && !cli.force {
            bail!(
                "世界文件已存在: {}，如需覆盖请加 --force",
                world_file.display()
            );
        }

        let prj_file = sidecar_path(path, "prj");
        if prj_file.exists() && !cli.force {
            bail!(
                "PRJ 文件已存在: {}，如需覆盖请加 --force",
                prj_file.display()
            );
        }
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

fn write_intensity_preview(
    preview_path: &PathBuf,
    resolution: f64,
    selected: &[SelectedPoint],
    header: &Header,
    force: bool,
) -> Result<()> {
    let layout = build_raster_layout(header, resolution)?;
    let mut raster = vec![RasterCell::default(); layout.pixel_count];

    for item in selected {
        accumulate_raster_point(&mut raster, &layout, &item.point);
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

    let col = (((x - layout.min_x) / layout.resolution).floor() as isize)
        .clamp(0, layout.width as isize - 1);
    let row = (((layout.max_y - y) / layout.resolution).floor() as isize)
        .clamp(0, layout.height as isize - 1);
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

    if let Some(parent) = preview_path.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent)
                .with_context(|| format!("无法创建目录: {}", parent.display()))?;
        }
    }

    image
        .save(preview_path)
        .with_context(|| format!("无法写出 PNG: {}", preview_path.display()))?;
    write_world_file(
        preview_path,
        layout.min_x,
        layout.max_y,
        layout.resolution,
        force,
    )?;
    write_prj_file(preview_path, header, force)?;
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
    let world_file = sidecar_path(preview_path, "pgw");
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
    let Some(wkt_bytes) = header.get_wkt_crs_bytes() else {
        return Ok(());
    };

    let prj_file = sidecar_path(preview_path, "prj");
    if prj_file.exists() && !force {
        bail!(
            "PRJ 文件已存在: {}，如需覆盖请加 --force",
            prj_file.display()
        );
    }

    fs::write(&prj_file, wkt_bytes)
        .with_context(|| format!("无法写出 PRJ 文件: {}", prj_file.display()))?;
    Ok(())
}

fn sidecar_path(path: &PathBuf, extension: &str) -> PathBuf {
    path.with_extension(extension)
}
