use anyhow::{Context, Result, bail};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

pub(crate) fn ensure_parent_dir(path: &Path) -> Result<()> {
    if let Some(parent) = path.parent()
        && !parent.as_os_str().is_empty()
    {
        fs::create_dir_all(parent)
            .with_context(|| format!("无法创建目录: {}", parent.display()))?;
    }
    Ok(())
}

pub(crate) fn validate_origin_file(path: &Path) -> Result<()> {
    let resolved = resolve_user_path(path)?;
    let content = fs::read_to_string(&resolved)
        .with_context(|| format!("无法读取 origin 文件: {}", resolved.display()))?;
    let parts: Vec<_> = content.split_whitespace().collect();
    if parts.len() < 3 {
        bail!("origin 文件至少需要包含 3 个值: lat lon alt");
    }
    for (idx, value) in parts.iter().take(3).enumerate() {
        value.parse::<f64>().with_context(|| {
            format!(
                "origin 第 {} 个字段不是有效浮点数: {}",
                idx + 1,
                resolved.display()
            )
        })?;
    }
    Ok(())
}

pub(crate) fn resolve_output_root(path: &Path) -> Result<PathBuf> {
    if path.exists() {
        return path
            .canonicalize()
            .with_context(|| format!("无法解析输出路径: {}", path.display()));
    }
    if path.is_absolute() {
        return Ok(path.to_path_buf());
    }
    Ok(std::env::current_dir()
        .context("无法获取当前工作目录")?
        .join(path))
}

pub(crate) fn resolve_user_path(path: &Path) -> Result<PathBuf> {
    if path.exists() {
        return path
            .canonicalize()
            .with_context(|| format!("无法解析路径: {}", path.display()));
    }
    if path.is_absolute() {
        return Ok(path.to_path_buf());
    }
    Ok(std::env::current_dir()
        .context("无法获取当前工作目录")?
        .join(path))
}

pub(crate) fn paths_equivalent(left: &Path, right: &Path) -> bool {
    let left = fs::canonicalize(left).unwrap_or_else(|_| left.to_path_buf());
    let right = fs::canonicalize(right).unwrap_or_else(|_| right.to_path_buf());
    left == right
}

pub(crate) fn system_time_to_epoch_ms(time: SystemTime) -> u64 {
    time.duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis()
        .try_into()
        .unwrap_or(u64::MAX)
}

pub(crate) fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub(crate) fn format_float(value: f64) -> String {
    let text = format!("{value:.6}");
    text.trim_end_matches('0').trim_end_matches('.').to_string()
}

pub(crate) fn sidecar_path(path: &Path, extension: &str) -> PathBuf {
    path.with_extension(extension)
}

pub(crate) fn world_file_path(path: &Path) -> PathBuf {
    sidecar_path(path, "pgw")
}

pub(crate) fn aux_xml_path(path: &Path) -> PathBuf {
    PathBuf::from(format!("{}.aux.xml", path.display()))
}

pub(crate) fn vrt_path(path: &Path) -> PathBuf {
    PathBuf::from(format!("{}.vrt", path.display()))
}

pub(crate) fn is_dataset_name(name: &str) -> bool {
    let Some((prefix, suffix)) = name.split_once("_out_source_") else {
        return false;
    };
    is_datetime_prefix(prefix)
        && (suffix.len() == 5 || suffix.len() == 6)
        && suffix.bytes().all(|byte| byte.is_ascii_digit())
}

fn is_datetime_prefix(value: &str) -> bool {
    if value.len() != 19 {
        return false;
    }
    for (idx, byte) in value.bytes().enumerate() {
        match idx {
            4 | 7 | 10 | 13 | 16 => {
                if byte != b'-' {
                    return false;
                }
            }
            _ => {
                if !byte.is_ascii_digit() {
                    return false;
                }
            }
        }
    }
    true
}
