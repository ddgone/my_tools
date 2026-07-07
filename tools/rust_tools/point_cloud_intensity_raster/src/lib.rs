use anyhow::{Context, Result, bail};
use clap::Parser;
use image::{
    ColorType, ImageEncoder, Rgba,
    codecs::png::{CompressionType, FilterType, PngEncoder},
};
use las::{Header, Point, Reader, crs::GeoTiffData};
use rayon::ThreadPoolBuilder;
use rayon::prelude::*;
use rustc_hash::FxHashMap;
use std::fs::{self, File};
use std::io::BufWriter;
use std::path::{Path, PathBuf};

const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;

#[derive(Debug, Clone, Parser)]
#[command(
    author,
    version,
    about = "对 LAS/LAZ 文件生成强度图，支持单文件或目录批量并发处理"
)]
struct Cli {
    #[arg(long, value_name = "PATH")]
    input: PathBuf,
    #[arg(long, value_name = "DIR")]
    output: PathBuf,
    #[arg(long, default_value_t = 0.5)]
    resolution: f64,
    #[arg(long, default_value_t = 4)]
    threads: usize,
    #[arg(long, default_value_t = false)]
    force: bool,
}

#[derive(Debug, Clone)]
struct Task {
    input_path: PathBuf,
    relative_path: PathBuf,
    output_png: PathBuf,
}

#[derive(Debug, Clone, Default)]
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

pub fn run(args: &[String]) -> Result<()> {
    let cli = Cli::try_parse_from(
        std::iter::once("point_cloud_intensity_raster".to_string()).chain(args.iter().cloned()),
    )?;
    run_cli(cli)
}

fn run_cli(cli: Cli) -> Result<()> {
    if !(cli.resolution.is_finite() && cli.resolution > 0.0) {
        bail!("resolution 必须是大于 0 的有限数值");
    }
    if cli.threads == 0 {
        bail!("threads 必须大于 0");
    }
    fs::create_dir_all(&cli.output)
        .with_context(|| format!("无法创建输出目录: {}", cli.output.display()))?;

    let tasks = discover_tasks(&cli.input, &cli.output)?;
    if tasks.is_empty() {
        bail!("未发现可处理的 LAS/LAZ 文件: {}", cli.input.display());
    }

    eprintln!(
        "[point_cloud_intensity_raster] 开始处理 {} 个文件，resolution={}，threads={}",
        tasks.len(),
        cli.resolution,
        cli.threads
    );

    let pool = ThreadPoolBuilder::new()
        .num_threads(cli.threads)
        .build()
        .context("创建 Rayon 线程池失败")?;
    pool.install(|| {
        tasks
            .par_iter()
            .try_for_each(|task| process_task(task, cli.resolution, cli.force))
    })?;

    eprintln!(
        "[point_cloud_intensity_raster] 已完成 {} 个文件，输出目录={}",
        tasks.len(),
        cli.output.display()
    );
    Ok(())
}

fn process_task(task: &Task, resolution: f64, force: bool) -> Result<()> {
    if task.output_png.exists() && !force {
        bail!(
            "输出文件已存在，如需覆盖请加 --force: {}",
            task.output_png.display()
        );
    }

    let mut reader = Reader::from_path(&task.input_path)
        .with_context(|| format!("无法打开输入文件: {}", task.input_path.display()))?;
    let header = reader.header().clone();
    let points = reader
        .points()
        .collect::<std::result::Result<Vec<_>, _>>()
        .with_context(|| format!("读取点记录失败: {}", task.input_path.display()))?;
    write_intensity_preview_points(&task.output_png, resolution, &points, &header, force)?;

    eprintln!(
        "[point_cloud_intensity_raster] {} -> {}",
        task.relative_path.display(),
        task.output_png.display()
    );
    Ok(())
}

fn discover_tasks(input: &Path, output_dir: &Path) -> Result<Vec<Task>> {
    if input.is_file() {
        if !is_supported_point_cloud_path(input) {
            bail!("输入文件不是 .las 或 .laz: {}", input.display());
        }
        let relative = PathBuf::from(input.file_name().context("无法获取输入文件名")?);
        return Ok(vec![Task {
            input_path: input.to_path_buf(),
            relative_path: relative.clone(),
            output_png: build_output_png(output_dir, &relative)?,
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
        output_under_input.as_deref(),
        &mut tasks,
    )?;
    tasks.sort_by(|left, right| left.relative_path.cmp(&right.relative_path));
    Ok(tasks)
}

fn collect_tasks_flat(
    input: &Path,
    output_dir: &Path,
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
            output_png: build_output_png(output_dir, &relative_path)?,
            relative_path,
        });
    }
    Ok(())
}

fn build_output_png(output_dir: &Path, relative_path: &Path) -> Result<PathBuf> {
    let stem = relative_path
        .file_stem()
        .and_then(|name| name.to_str())
        .context("无法获取输入文件名")?;
    let output_path = output_dir.join(stem).join(format!("{stem}_intensity.png"));
    Ok(output_path)
}

fn write_intensity_preview_points(
    preview_path: &Path,
    resolution: f64,
    points: &[Point],
    header: &Header,
    force: bool,
) -> Result<()> {
    let layout = build_raster_layout(header, resolution)?;
    let raster = accumulate_raster_points_parallel(points, &layout);
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
            "栅格过大: {} x {}，请增大 --resolution 后重试",
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

fn accumulate_raster_points_parallel(points: &[Point], layout: &RasterLayout) -> Vec<RasterCell> {
    if points.is_empty() {
        return vec![RasterCell::default(); layout.pixel_count];
    }
    let threads = rayon::current_num_threads().max(1);
    let chunk_size = (points.len() / threads).clamp(50_000, 500_000);
    let sparse = points
        .par_chunks(chunk_size)
        .fold(FxHashMap::<usize, RasterCell>::default, |mut acc, chunk| {
            for point in chunk {
                let Some(idx) = raster_index(layout, point) else {
                    continue;
                };
                let cell = acc.entry(idx).or_default();
                cell.intensity_sum += f64::from(point.intensity);
                cell.count += 1;
            }
            acc
        })
        .reduce(
            FxHashMap::<usize, RasterCell>::default,
            |mut left, right| {
                if left.len() < right.len() {
                    let mut right = right;
                    for (idx, cell) in left {
                        let target = right.entry(idx).or_default();
                        target.intensity_sum += cell.intensity_sum;
                        target.count += cell.count;
                    }
                    right
                } else {
                    for (idx, cell) in right {
                        let target = left.entry(idx).or_default();
                        target.intensity_sum += cell.intensity_sum;
                        target.count += cell.count;
                    }
                    left
                }
            },
        );
    let mut raster = vec![RasterCell::default(); layout.pixel_count];
    for (idx, cell) in sparse {
        raster[idx] = cell;
    }
    raster
}

fn raster_index(layout: &RasterLayout, point: &Point) -> Option<usize> {
    let x = point.x;
    let y = point.y;
    if x < layout.min_x || x > layout.max_x || y < layout.min_y || y > layout.max_y {
        return None;
    }
    let col = (((x - layout.min_x) / layout.resolution).floor() as isize)
        .clamp(0, layout.width as isize - 1);
    let row = (((layout.max_y - y) / layout.resolution).floor() as isize)
        .clamp(0, layout.height as isize - 1);
    Some(row as usize * layout.width + col as usize)
}

fn write_intensity_preview_from_raster(
    preview_path: &Path,
    layout: &RasterLayout,
    raster: &[RasterCell],
    header: &Header,
    force: bool,
) -> Result<()> {
    let mut values = Vec::with_capacity(raster.len());
    for cell in raster {
        if cell.count > 0 {
            values.push(cell.intensity_sum / f64::from(cell.count));
        }
    }
    if values.is_empty() {
        bail!("没有可用于生成强度图的有效像素");
    }
    let (low, high) = intensity_window_from_values(&mut values);
    let mut raw = vec![0_u8; layout.pixel_count * 4];
    raw.par_chunks_mut(4).enumerate().for_each(|(idx, pixel)| {
        let cell = &raster[idx];
        if cell.count == 0 {
            pixel.copy_from_slice(&[0, 0, 0, 0]);
            return;
        }
        let value = cell.intensity_sum / f64::from(cell.count);
        let gray = normalize_to_u8(value, low, high);
        pixel.copy_from_slice(&Rgba([gray, gray, gray, 255]).0);
    });

    ensure_parent_dir(preview_path)?;
    if preview_path.exists() && !force {
        bail!(
            "强度图已存在，如需覆盖请加 --force: {}",
            preview_path.display()
        );
    }
    let file = File::create(preview_path)
        .with_context(|| format!("无法写出强度预览图: {}", preview_path.display()))?;
    let writer = BufWriter::new(file);
    let encoder = PngEncoder::new_with_quality(writer, CompressionType::Fast, FilterType::NoFilter);
    encoder
        .write_image(
            &raw,
            layout.width as u32,
            layout.height as u32,
            ColorType::Rgba8.into(),
        )
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

fn intensity_window_from_values(values: &mut [f64]) -> (f64, f64) {
    let last = values.len() - 1;
    let low_idx = ((last as f64) * 0.02).round() as usize;
    let high_idx = ((last as f64) * 0.98).round() as usize;
    let (_, low_ref, upper) = values.select_nth_unstable_by(low_idx, |a, b| a.total_cmp(b));
    let low = *low_ref;
    if high_idx <= low_idx {
        return (low, low + 1.0);
    }
    let rel_idx = (high_idx - low_idx - 1).min(upper.len().saturating_sub(1));
    let (_, high_ref, _) = upper.select_nth_unstable_by(rel_idx, |a, b| a.total_cmp(b));
    (low, (*high_ref).max(low + 1.0))
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
    preview_path: &Path,
    min_x: f64,
    max_y: f64,
    resolution: f64,
    force: bool,
) -> Result<()> {
    let world_file = world_file_path(preview_path)?;
    if world_file.exists() && !force {
        bail!(
            "世界文件已存在，如需覆盖请加 --force: {}",
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

fn write_prj_file(preview_path: &Path, header: &Header, force: bool) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };
    let prj_file = sidecar_path(preview_path, "prj");
    if prj_file.exists() && !force {
        bail!(
            "PRJ 文件已存在，如需覆盖请加 --force: {}",
            prj_file.display()
        );
    }
    fs::write(&prj_file, wkt)
        .with_context(|| format!("无法写出 PRJ 文件: {}", prj_file.display()))?;
    Ok(())
}

fn write_aux_xml_file(
    preview_path: &Path,
    layout: &RasterLayout,
    header: &Header,
    force: bool,
) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };
    let aux_xml = aux_xml_path(preview_path);
    if aux_xml.exists() && !force {
        bail!(
            "Aux XML 文件已存在，如需覆盖请加 --force: {}",
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

fn write_vrt_file(
    preview_path: &Path,
    layout: &RasterLayout,
    header: &Header,
    force: bool,
) -> Result<()> {
    let Some(wkt) = header_crs_wkt(header)? else {
        return Ok(());
    };
    let vrt_path = vrt_path(preview_path);
    if vrt_path.exists() && !force {
        bail!(
            "VRT 文件已存在，如需覆盖请加 --force: {}",
            vrt_path.display()
        );
    }
    let source_name = preview_path
        .file_name()
        .and_then(|name| name.to_str())
        .context("无法获取 PNG 文件名以生成 VRT")?;
    let geotransform = format!(
        "{:.12}, {:.12}, 0.0, {:.12}, 0.0, -{:.12}",
        layout.min_x, layout.resolution, layout.max_y, layout.resolution
    );
    let mut xml = String::new();
    xml.push_str(&format!(
        "<VRTDataset rasterXSize=\"{}\" rasterYSize=\"{}\">\n",
        layout.width, layout.height
    ));
    xml.push_str(&format!("  <SRS>{}</SRS>\n", xml_escape(&wkt)));
    xml.push_str(&format!(
        "  <GeoTransform>{}</GeoTransform>\n",
        geotransform
    ));
    for band in 1..=4 {
        xml.push_str(&format!(
            "  <VRTRasterBand dataType=\"Byte\" band=\"{}\">\n",
            band
        ));
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
            layout.width, layout.height, layout.width
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
    fs::write(&vrt_path, xml)
        .with_context(|| format!("无法写出 VRT 文件: {}", vrt_path.display()))?;
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

fn build_utm_wkt(epsg: u16) -> Result<String> {
    let (zone, northern) = epsg_to_utm_zone(epsg)?;
    let central_meridian = (i32::from(zone) - 1) * 6 - 180 + 3;
    let false_northing = if northern {
        0.0
    } else {
        UTM_FALSE_NORTHING_SOUTH
    };
    let hemisphere = if northern { "N" } else { "S" };
    Ok(format!(
        "PROJCS[\"WGS 84 / UTM zone {zone}{hemisphere}\",GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563]],PRIMEM[\"Greenwich\",0],UNIT[\"degree\",0.0174532925199433]],PROJECTION[\"Transverse_Mercator\"],PARAMETER[\"latitude_of_origin\",0],PARAMETER[\"central_meridian\",{central_meridian}],PARAMETER[\"scale_factor\",0.9996],PARAMETER[\"false_easting\",500000],PARAMETER[\"false_northing\",{false_northing}],UNIT[\"metre\",1]]"
    ))
}

fn epsg_to_utm_zone(epsg: u16) -> Result<(u8, bool)> {
    match epsg {
        32601..=32660 => Ok(((epsg - 32600) as u8, true)),
        32701..=32760 => Ok(((epsg - 32700) as u8, false)),
        _ => bail!("目前仅支持 WGS84 UTM EPSG:32601-32660 或 EPSG:32701-32760"),
    }
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

fn sidecar_path(path: &Path, extension: &str) -> PathBuf {
    path.with_extension(extension)
}

fn world_file_path(path: &Path) -> Result<PathBuf> {
    preview_format(path)?;
    Ok(sidecar_path(path, "pgw"))
}

fn preview_format(path: &Path) -> Result<&'static str> {
    let ext = path
        .extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| ext.to_ascii_lowercase())
        .context("强度图输出路径需要带文件扩展名")?;
    match ext.as_str() {
        "png" => Ok("png"),
        _ => bail!("目前仅支持输出 .png"),
    }
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

fn is_supported_point_cloud_path(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .map(|ext| matches!(ext.to_ascii_lowercase().as_str(), "las" | "laz"))
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::{build_output_png, is_supported_point_cloud_path};
    use std::path::Path;

    #[test]
    fn should_recognize_supported_extensions_case_insensitively() {
        assert!(is_supported_point_cloud_path(Path::new("a.las")));
        assert!(is_supported_point_cloud_path(Path::new("a.LAZ")));
        assert!(!is_supported_point_cloud_path(Path::new("a.txt")));
    }

    #[test]
    fn should_build_png_output_name() {
        let path =
            build_output_png(Path::new("D:/out"), Path::new("sample.laz")).expect("png");
        assert_eq!(path, Path::new("D:/out/sample/sample_intensity.png"));
    }
}
