use crate::legacy::io_utils::ensure_parent_dir;
use crate::legacy::transform::build_utm_wkt;
use crate::legacy::types::{PreviewBuildStats, RasterCell, RasterLayout};
use anyhow::{Context, Result, bail};
use image::{
    ColorType, ImageEncoder, Rgba,
    codecs::png::{CompressionType, FilterType, PngEncoder},
};
use las::{Header, Point, crs::GeoTiffData};
use rayon::prelude::*;
use rustc_hash::FxHashMap;
use std::fs::{self, File};
use std::io::BufWriter;
use std::path::{Path, PathBuf};

pub(crate) fn write_intensity_preview_points(
    preview_path: &PathBuf,
    resolution: f64,
    points: &[Point],
    header: &Header,
    force: bool,
) -> Result<PreviewBuildStats> {
    let total_started = std::time::Instant::now();
    let layout = build_raster_layout(header, resolution)?;
    let accumulate_started = std::time::Instant::now();
    let raster = accumulate_raster_points_parallel(points, &layout);
    let accumulate_secs = accumulate_started.elapsed().as_secs_f64();

    let mut stats =
        write_intensity_preview_from_raster(preview_path, &layout, &raster, header, force)?;
    stats.width = layout.width;
    stats.height = layout.height;
    stats.accumulate_secs = accumulate_secs;
    stats.total_secs = total_started.elapsed().as_secs_f64();
    Ok(stats)
}

pub(crate) fn build_raster_layout(header: &Header, resolution: f64) -> Result<RasterLayout> {
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

pub(crate) fn accumulate_raster_points_parallel(
    points: &[Point],
    layout: &RasterLayout,
) -> Vec<RasterCell> {
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

pub(crate) fn write_intensity_preview_from_raster(
    preview_path: &PathBuf,
    layout: &RasterLayout,
    raster: &[RasterCell],
    header: &Header,
    force: bool,
) -> Result<PreviewBuildStats> {
    let mut stats = PreviewBuildStats {
        width: layout.width,
        height: layout.height,
        ..PreviewBuildStats::default()
    };
    let quantile_started = std::time::Instant::now();
    let mut values = Vec::with_capacity(raster.len());
    for cell in raster {
        if cell.count > 0 {
            values.push(cell.intensity_sum / f64::from(cell.count));
        }
    }
    if values.is_empty() {
        bail!("没有可用于生成强度图的有效像素");
    }
    stats.non_empty_pixels = values.len();
    let (low, high) = intensity_window_from_values(&mut values);
    stats.quantile_secs = quantile_started.elapsed().as_secs_f64();

    let render_started = std::time::Instant::now();
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
    stats.render_secs = render_started.elapsed().as_secs_f64();

    let encode_started = std::time::Instant::now();
    ensure_parent_dir(preview_path)?;
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
    stats.encode_secs = encode_started.elapsed().as_secs_f64();

    let sidecar_started = std::time::Instant::now();
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
    stats.sidecar_secs = sidecar_started.elapsed().as_secs_f64();
    Ok(stats)
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

fn write_aux_xml_file(
    preview_path: &PathBuf,
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

fn write_vrt_file(
    preview_path: &PathBuf,
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

pub(crate) fn sidecar_path(path: &Path, extension: &str) -> PathBuf {
    path.with_extension(extension)
}

pub(crate) fn preview_format(path: &Path) -> Result<&'static str> {
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

pub(crate) fn world_file_path(path: &Path) -> Result<PathBuf> {
    preview_format(path)?;
    Ok(sidecar_path(path, "pgw"))
}

pub(crate) fn aux_xml_path(path: &Path) -> PathBuf {
    PathBuf::from(format!("{}.aux.xml", path.display()))
}

pub(crate) fn vrt_path(path: &Path) -> PathBuf {
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
