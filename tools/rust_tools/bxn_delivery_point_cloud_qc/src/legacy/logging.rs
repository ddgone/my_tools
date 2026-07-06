use crate::legacy::io_utils::ensure_parent_dir;
use anyhow::{Context, Result};
use std::fs::{File, OpenOptions};
use std::io::{BufWriter, Write};
use std::path::Path;
use std::sync::{Arc, Mutex};

pub(crate) struct ProcessLogger {
    file: Option<BufWriter<File>>,
    quiet: bool,
}

impl ProcessLogger {
    pub(crate) fn new(log_path: Option<&Path>, quiet: bool) -> Result<Self> {
        let file = match log_path {
            Some(path) => {
                ensure_parent_dir(path)?;
                let file = OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(path)
                    .with_context(|| format!("无法打开日志文件: {}", path.display()))?;
                Some(BufWriter::new(file))
            }
            None => None,
        };
        Ok(Self { file, quiet })
    }

    pub(crate) fn log(&mut self, level: &str, message: impl AsRef<str>) {
        let line = format!("[{}] {}", level, message.as_ref());
        if !self.quiet || level != "DEBUG" {
            eprintln!("{line}");
        }
        if let Some(file) = &mut self.file {
            let _ = writeln!(file, "{line}");
            let _ = file.flush();
        }
    }
}

pub(crate) fn log_process(
    logger: &Arc<Mutex<ProcessLogger>>,
    level: &str,
    message: impl AsRef<str>,
) {
    if let Ok(mut guard) = logger.lock() {
        guard.log(level, message);
    }
}
