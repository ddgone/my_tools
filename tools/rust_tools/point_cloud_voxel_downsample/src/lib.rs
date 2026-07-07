use anyhow::{Context, Result, bail};
use clap::{Parser, ValueEnum};
use las::{Builder, Header, Point, Reader, Writer, laz};
use rayon::ThreadPoolBuilder;
use rayon::prelude::*;
use rustc_hash::FxHashMap;
use std::collections::hash_map::Entry;
use std::fs::{self, File};
use std::io::BufWriter;
use std::path::{Path, PathBuf};

const WRITE_CHUNK_POINTS: usize = 250_000;

#[derive(Debug, Clone, Parser)]
#[command(
    author,
    version,
    about = "对 LAS/LAZ 文件执行体素抽稀，支持单文件或目录批量并发处理"
)]
struct Cli {
    #[arg(long, value_name = "PATH")]
    input: PathBuf,
    #[arg(long, value_name = "DIR")]
    output: PathBuf,
    #[arg(long, default_value_t = 0.2)]
    voxel_size: f64,
    #[arg(long, value_enum, default_value_t = RepresentativeMode::Center)]
    representative: RepresentativeMode,
    #[arg(long, value_enum, default_value_t = OutputFormat::Preserve)]
    output_format: OutputFormat,
    #[arg(long, default_value_t = 4)]
    threads: usize,
    #[arg(long, default_value_t = false)]
    force: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, ValueEnum)]
enum RepresentativeMode {
    First,
    Center,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, ValueEnum)]
enum OutputFormat {
    Preserve,
    Laz,
    Las,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
struct VoxelKey {
    x: i32,
    y: i32,
    z: i32,
}

#[derive(Debug, Clone)]
struct SelectedPoint {
    key: VoxelKey,
    point: Point,
    score: f64,
    order: u64,
}

#[derive(Debug, Clone)]
struct Task {
    input_path: PathBuf,
    relative_path: PathBuf,
    output_path: PathBuf,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = Cli::try_parse_from(
        std::iter::once("point_cloud_voxel_downsample".to_string()).chain(args.iter().cloned()),
    )?;
    run_cli(cli)
}

fn run_cli(cli: Cli) -> Result<()> {
    if !(cli.voxel_size.is_finite() && cli.voxel_size >= 0.0) {
        bail!("voxel-size 必须是大于等于 0 的有限数值");
    }
    if cli.threads == 0 {
        bail!("threads 必须大于 0");
    }
    fs::create_dir_all(&cli.output)
        .with_context(|| format!("无法创建输出目录: {}", cli.output.display()))?;

    let tasks = discover_tasks(&cli.input, &cli.output, cli.output_format, cli.voxel_size)?;
    if tasks.is_empty() {
        bail!("未发现可处理的 LAS/LAZ 文件: {}", cli.input.display());
    }

    eprintln!(
        "[point_cloud_voxel_downsample] 开始处理 {} 个文件，voxel_size={}，representative={:?}，output_format={:?}，threads={}",
        tasks.len(),
        cli.voxel_size,
        cli.representative,
        cli.output_format,
        cli.threads
    );

    let pool = ThreadPoolBuilder::new()
        .num_threads(cli.threads)
        .build()
        .context("创建 Rayon 线程池失败")?;

    pool.install(|| {
        tasks
            .par_iter()
            .try_for_each(|task| process_task(task, cli.voxel_size, cli.representative, cli.force))
    })?;

    eprintln!(
        "[point_cloud_voxel_downsample] 已完成 {} 个文件，输出目录={}",
        tasks.len(),
        cli.output.display()
    );
    Ok(())
}

fn process_task(
    task: &Task,
    voxel_size: f64,
    representative: RepresentativeMode,
    force: bool,
) -> Result<()> {
    if task.output_path.exists() && !force {
        bail!(
            "输出文件已存在，如需覆盖请加 --force: {}",
            task.output_path.display()
        );
    }

    let mut reader = Reader::from_path(&task.input_path)
        .with_context(|| format!("无法打开输入文件: {}", task.input_path.display()))?;
    let header = reader.header().clone();
    let points = reader
        .points()
        .collect::<std::result::Result<Vec<_>, _>>()
        .with_context(|| format!("读取点记录失败: {}", task.input_path.display()))?;
    let input_count = points.len();

    let selected_points = downsample_points(&points, voxel_size, representative)?;
    let output_header = rebuild_header_for_points(&header, &selected_points)?;
    write_point_cloud(&task.output_path, &output_header, &selected_points)
        .with_context(|| format!("写出失败: {}", task.output_path.display()))?;

    eprintln!(
        "[point_cloud_voxel_downsample] {} -> {} ({} -> {})",
        task.relative_path.display(),
        task.output_path.display(),
        input_count,
        selected_points.len()
    );
    Ok(())
}

fn discover_tasks(
    input: &Path,
    output_dir: &Path,
    output_format: OutputFormat,
    voxel_size: f64,
) -> Result<Vec<Task>> {
    if input.is_file() {
        if !is_supported_point_cloud_path(input) {
            bail!("输入文件不是 .las 或 .laz: {}", input.display());
        }
        let relative = PathBuf::from(input.file_name().context("无法获取输入文件名")?);
        return Ok(vec![Task {
            input_path: input.to_path_buf(),
            relative_path: relative.clone(),
            output_path: build_output_path(output_dir, &relative, output_format, voxel_size)?,
        }]);
    }
    if !input.is_dir() {
        bail!("输入路径不存在或既不是文件也不是目录: {}", input.display());
    }

    let mut tasks = Vec::new();
    let output_under_input = output_dir
        .canonicalize()
        .ok()
        .filter(|canonical| canonical.starts_with(input));
    collect_tasks_flat(
        input,
        output_dir,
        output_format,
        voxel_size,
        output_under_input.as_deref(),
        &mut tasks,
    )?;
    tasks.sort_by(|left, right| left.relative_path.cmp(&right.relative_path));
    Ok(tasks)
}

fn collect_tasks_flat(
    input: &Path,
    output_dir: &Path,
    output_format: OutputFormat,
    voxel_size: f64,
    output_under_input: Option<&Path>,
    tasks: &mut Vec<Task>,
) -> Result<()> {
    for entry in
        fs::read_dir(input).with_context(|| format!("无法读取目录: {}", input.display()))?
    {
        let entry = entry.with_context(|| format!("无法读取目录项: {}", input.display()))?;
        let path = entry.path();
        if let Some(output_dir) = output_under_input
            && path.starts_with(output_dir)
        {
            continue;
        }
        let file_type = entry
            .file_type()
            .with_context(|| format!("无法识别路径类型: {}", path.display()))?;
        if !file_type.is_file() || !is_supported_point_cloud_path(&path) {
            continue;
        }
        let relative_path = PathBuf::from(
            path.file_name().context("无法获取输入文件名")?,
        );
        tasks.push(Task {
            input_path: path.clone(),
            output_path: build_output_path(output_dir, &relative_path, output_format, voxel_size)?,
            relative_path,
        });
    }
    Ok(())
}

fn build_output_path(
    output_dir: &Path,
    relative_path: &Path,
    output_format: OutputFormat,
    voxel_size: f64,
) -> Result<PathBuf> {
    let stem = relative_path
        .file_stem()
        .and_then(|name| name.to_str())
        .context("无法获取输入文件名")?;
    let extension = match output_format {
        OutputFormat::Preserve => relative_path
            .extension()
            .and_then(|ext| ext.to_str())
            .map(|ext| ext.to_ascii_lowercase())
            .context("无法获取输入文件扩展名")?,
        OutputFormat::Laz => "laz".to_string(),
        OutputFormat::Las => "las".to_string(),
    };
    let suffix = if voxel_size > 0.0 {
        format!("_voxel_{}", format_float(voxel_size))
    } else {
        "_voxel_all".to_string()
    };
    let file_name = format!("{stem}{suffix}.{extension}");
    let mut output_path = output_dir.join(relative_path);
    output_path.set_file_name(file_name);
    Ok(output_path)
}

fn downsample_points(
    points: &[Point],
    voxel_size: f64,
    representative: RepresentativeMode,
) -> Result<Vec<Point>> {
    if voxel_size <= 0.0 || points.is_empty() {
        return Ok(points.to_vec());
    }
    let inv_voxel = 1.0 / voxel_size;
    let threads = rayon::current_num_threads().max(1);
    let chunk_size = (points.len() / threads).clamp(50_000, 500_000);

    let partials = points
        .par_chunks(chunk_size)
        .enumerate()
        .map(|(chunk_index, chunk)| -> Result<Vec<SelectedPoint>> {
            let mut voxel_index = FxHashMap::<VoxelKey, usize>::default();
            let mut selected = Vec::<SelectedPoint>::new();
            let base_order = chunk_index as u64 * chunk_size as u64;
            for (offset, point) in chunk.iter().enumerate() {
                let key = voxel_key(point, inv_voxel)?;
                insert_selected_point_with_key(
                    &mut voxel_index,
                    &mut selected,
                    key,
                    point.clone(),
                    voxel_size,
                    representative,
                    base_order + offset as u64,
                );
            }
            Ok(selected)
        })
        .collect::<Result<Vec<_>>>()?;

    let mut merged_index = FxHashMap::<VoxelKey, usize>::default();
    let mut merged_selected = Vec::<SelectedPoint>::new();
    for partial in partials {
        for selected in partial {
            insert_selected_point_with_key(
                &mut merged_index,
                &mut merged_selected,
                selected.key,
                selected.point,
                voxel_size,
                representative,
                selected.order,
            );
        }
    }
    Ok(merged_selected.into_iter().map(|item| item.point).collect())
}

fn insert_selected_point_with_key(
    voxel_index: &mut FxHashMap<VoxelKey, usize>,
    selected: &mut Vec<SelectedPoint>,
    key: VoxelKey,
    point: Point,
    voxel_size: f64,
    representative: RepresentativeMode,
    order: u64,
) {
    let score = score_point(&point, key, voxel_size, representative);
    match voxel_index.entry(key) {
        Entry::Occupied(entry) => {
            let idx = *entry.get();
            let replace = match representative {
                RepresentativeMode::First => order < selected[idx].order,
                RepresentativeMode::Center => score < selected[idx].score,
            };
            if replace {
                selected[idx] = SelectedPoint {
                    key,
                    point,
                    score,
                    order,
                };
            }
        }
        Entry::Vacant(entry) => {
            let idx = selected.len();
            entry.insert(idx);
            selected.push(SelectedPoint {
                key,
                point,
                score,
                order,
            });
        }
    }
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

fn score_point(
    point: &Point,
    key: VoxelKey,
    voxel_size: f64,
    representative: RepresentativeMode,
) -> f64 {
    match representative {
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

fn write_point_cloud(path: &Path, header: &Header, points: &[Point]) -> Result<()> {
    ensure_parent_dir(path)?;
    let output_header = sanitize_header_for_write(header)?;
    let compress = path
        .extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| ext.eq_ignore_ascii_case("laz"))
        .unwrap_or(false);
    let mut output_header_builder = Builder::from(output_header);
    output_header_builder.point_format.is_compressed = compress;
    let output_header = output_header_builder
        .into_header()
        .context("构建输出 header 失败")?;
    let file =
        File::create(path).with_context(|| format!("无法创建输出文件: {}", path.display()))?;
    let buffer = BufWriter::with_capacity(16 * 1024 * 1024, file);
    let mut writer = Writer::new(buffer, output_header)
        .with_context(|| format!("无法创建输出文件: {}", path.display()))?;
    for chunk in points.chunks(WRITE_CHUNK_POINTS) {
        writer
            .write_points(chunk)
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

fn is_supported_point_cloud_path(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| matches!(ext.to_ascii_lowercase().as_str(), "las" | "laz"))
        .unwrap_or(false)
}

fn format_float(value: f64) -> String {
    let mut text = format!("{value:.6}");
    while text.contains('.') && text.ends_with('0') {
        text.pop();
    }
    if text.ends_with('.') {
        text.pop();
    }
    if text.is_empty() {
        "0".to_string()
    } else {
        text
    }
}

#[cfg(test)]
mod tests {
    use super::{OutputFormat, build_output_path, format_float, is_supported_point_cloud_path};
    use std::path::Path;

    #[test]
    fn should_recognize_supported_extensions_case_insensitively() {
        assert!(is_supported_point_cloud_path(Path::new("a.las")));
        assert!(is_supported_point_cloud_path(Path::new("a.LAZ")));
        assert!(!is_supported_point_cloud_path(Path::new("a.txt")));
    }

    #[test]
    fn should_build_output_path_with_suffix() {
        let path = build_output_path(
            Path::new("D:/out"),
            Path::new("nested/sample.laz"),
            OutputFormat::Las,
            0.2,
        )
        .expect("build output path");
        assert_eq!(path, Path::new("D:/out/nested/sample_voxel_0.2.las"));
    }

    #[test]
    fn should_trim_float_for_output_names() {
        assert_eq!(format_float(0.200000), "0.2");
        assert_eq!(format_float(1.0), "1");
    }
}
