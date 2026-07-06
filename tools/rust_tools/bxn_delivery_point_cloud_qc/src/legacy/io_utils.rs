use anyhow::{Context, Result};
use las::{Builder, Header, Point, Writer, laz};
use std::fs::{self, File};
use std::io::BufWriter;
use std::path::Path;

pub(crate) fn write_point_cloud(
    path: &Path,
    header: &Header,
    points: &[Point],
    _force: bool,
) -> Result<()> {
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
    const LAZ_WRITE_CHUNK_POINTS: usize = 250_000;
    for chunk in points.chunks(LAZ_WRITE_CHUNK_POINTS) {
        writer
            .write_points(chunk)
            .with_context(|| format!("写出点记录失败: {}", path.display()))?;
    }
    Ok(())
}

pub(crate) fn ensure_parent_dir(path: &Path) -> Result<()> {
    if let Some(parent) = path.parent()
        && !parent.as_os_str().is_empty()
    {
        fs::create_dir_all(parent)
            .with_context(|| format!("无法创建目录: {}", parent.display()))?;
    }
    Ok(())
}

pub(crate) fn sanitize_header_for_write(header: &Header) -> Result<Header> {
    let mut builder = Builder::from(header.clone());
    builder.vlrs.retain(|vlr| !laz::is_laszip_vlr(vlr));
    builder.into_header().context("清理写出 header 失败")
}
