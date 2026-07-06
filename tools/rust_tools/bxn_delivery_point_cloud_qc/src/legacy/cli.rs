use anyhow::{Context, Result, bail};
use rayon::ThreadPoolBuilder;
use std::path::Path;
use std::sync::OnceLock;

static RAYON_THREADS: OnceLock<Option<usize>> = OnceLock::new();

pub(crate) fn validate_output_path(path: &Path, force: bool, label: &str) -> Result<()> {
    if path.exists() && !force {
        bail!("{}已存在: {}，如需覆盖请加 --force", label, path.display());
    }
    Ok(())
}

pub(crate) fn configure_threads(threads: Option<usize>) -> Result<()> {
    if let Some(existing) = RAYON_THREADS.get() {
        if *existing != threads {
            bail!(
                "rayon 线程池已经初始化为 {:?}，当前任务要求 {:?}，请保持整个批次使用同一个 --threads 参数",
                existing,
                threads
            );
        }
        return Ok(());
    }
    if let Some(threads) = threads {
        ThreadPoolBuilder::new()
            .num_threads(threads)
            .build_global()
            .context("初始化 rayon 线程池失败")?;
    }
    let _ = RAYON_THREADS.set(threads);
    Ok(())
}
