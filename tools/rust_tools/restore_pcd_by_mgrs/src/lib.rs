//! 白犀牛偏转后的 PCD 文件按文件名 MGRS 百米块信息还原为 UTM 坐标 LAS。

mod mgrs;
mod pcd;

use anyhow::{Context, Result, bail};
use clap::Parser;
use rayon::prelude::*;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Instant;

#[derive(Parser)]
#[command(
    name = "restore_pcd_by_mgrs",
    version,
    about = "白犀牛偏转后的 PCD 按文件名 MGRS 百米块还原为 UTM 坐标 LAS"
)]
struct Cli {
    /// PCD 输入（目录或单个 .pcd 文件）
    #[arg(short, long, value_name = "PATH")]
    input: PathBuf,

    /// LAS 输出目录，默认在输入目录下创建 output 子目录
    #[arg(short, long, value_name = "DIR")]
    output: Option<PathBuf>,

    /// 并发线程数
    #[arg(short, long, default_value_t = 4)]
    threads: usize,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = Cli::try_parse_from(
        std::iter::once("restore_pcd_by_mgrs").chain(args.iter().map(String::as_str)),
    )?;

    let input_files = collect_pcd_files(&cli.input)?;
    if input_files.is_empty() {
        bail!("输入中未找到 .pcd 文件: {}", cli.input.display());
    }

    let output_dir = cli
        .output
        .clone()
        .unwrap_or_else(|| default_output_dir(&cli.input));
    fs::create_dir_all(&output_dir)
        .with_context(|| format!("无法创建输出目录: {}", output_dir.display()))?;

    eprintln!("输入: {}", cli.input.display());
    eprintln!("输出: {}", output_dir.display());
    eprintln!("共 {} 个 PCD 文件", input_files.len());

    let start = Instant::now();
    let convert = || -> Vec<Result<String>> {
        input_files
            .par_iter()
            .map(|path| process_one(path, &output_dir))
            .collect()
    };
    // 用局部线程池而非全局池，避免同进程多次调用时重复初始化 panic。
    let results: Vec<Result<String>> = if cli.threads > 0 {
        rayon::ThreadPoolBuilder::new()
            .num_threads(cli.threads)
            .build()
            .context("初始化线程池失败")?
            .install(convert)
    } else {
        convert()
    };

    let total = results.len();
    let ok = results.iter().filter(|r| r.is_ok()).count();
    let fail = total - ok;
    for (path, result) in input_files.iter().zip(results) {
        match result {
            Ok(msg) => eprintln!("OK    {msg}"),
            Err(e) => eprintln!("FAIL  {} - {e:#}", path.display()),
        }
    }
    eprintln!(
        "完成: 成功 {ok}, 失败 {fail}, 耗时 {:.1}s",
        start.elapsed().as_secs_f64()
    );
    if ok == 0 && fail > 0 {
        bail!("全部失败");
    }
    Ok(())
}

fn collect_pcd_files(input: &Path) -> Result<Vec<PathBuf>> {
    if input.is_dir() {
        let mut files = Vec::new();
        for entry in fs::read_dir(input)
            .with_context(|| format!("无法读取目录: {}", input.display()))?
        {
            let entry = entry?;
            if !entry.file_type()?.is_file() {
                continue;
            }
            let path = entry.path();
            if path.extension().and_then(|e| e.to_str()) == Some("pcd") {
                files.push(path);
            }
        }
        files.sort();
        Ok(files)
    } else if input.is_file() {
        Ok(vec![input.to_path_buf()])
    } else {
        bail!("无效的输入路径: {}", input.display())
    }
}

fn default_output_dir(input: &Path) -> PathBuf {
    if input.is_dir() {
        input.join("output")
    } else {
        input.parent().unwrap_or(Path::new(".")).join("output")
    }
}

fn process_one(pcd_path: &Path, output_dir: &Path) -> Result<String> {
    let filename = pcd_path
        .file_name()
        .and_then(|n| n.to_str())
        .context("无法获取文件名")?;
    let stem = pcd_path
        .file_stem()
        .and_then(|s| s.to_str())
        .context("无法获取文件名主干")?;

    let offset = mgrs::parse_mgrs_offset(filename)
        .with_context(|| format!("文件名不是有效的 MGRS 百米块格式: {filename}"))?;

    let points = pcd::read_pcd_points(pcd_path)?;
    if points.is_empty() {
        bail!("点云为空");
    }

    let las_path = output_dir.join(format!("{stem}_restored.las"));
    pcd::write_las(&las_path, &points, &offset)?;
    Ok(format!(
        "{stem}.pcd -> {stem}_restored.las ({} 点, EPSG:{}, 偏移 +{} +{})",
        points.len(),
        offset.epsg,
        offset.offset_x,
        offset.offset_y
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    fn write_test_pcd(path: &Path) {
        let header = "\
# .PCD v0.7 - Point Cloud Data file format
VERSION 0.7
FIELDS x y z rgb
SIZE 4 4 4 4
TYPE F F F F
COUNT 1 1 1 1
WIDTH 2
HEIGHT 1
VIEWPOINT 0 0 0 1 0 0 0
POINTS 2
DATA binary
";
        let mut file = fs::File::create(path).unwrap();
        file.write_all(header.as_bytes()).unwrap();
        for &(x, y, z, rgb) in &[
            (1.5_f32, 2.5_f32, 3.5_f32, 0x102030_u32),
            (10.25, 20.5, 30.75, 0),
        ] {
            file.write_all(&x.to_le_bytes()).unwrap();
            file.write_all(&y.to_le_bytes()).unwrap();
            file.write_all(&z.to_le_bytes()).unwrap();
            file.write_all(&f32::from_bits(rgb).to_le_bytes()).unwrap();
        }
    }

    #[test]
    fn converts_full_directory_end_to_end() {
        let dir = std::env::temp_dir().join("restore_pcd_by_mgrs_test_e2e");
        let input = dir.join("input");
        fs::create_dir_all(&input).unwrap();
        write_test_pcd(&input.join("50QKL416457.pcd"));
        // 无效文件名应计入失败但不妨碍其余文件
        write_test_pcd(&input.join("plain.pcd"));

        let output = dir.join("las_out");
        run(&[
            "--input".to_string(),
            input.to_string_lossy().into_owned(),
            "--output".to_string(),
            output.to_string_lossy().into_owned(),
            "--threads".to_string(),
            "1".to_string(),
        ])
        .unwrap();

        let las_path = output.join("50QKL416457_restored.las");
        assert!(las_path.exists());
        assert!(!output.join("plain_restored.las").exists());

        let mut reader = las::Reader::from_path(&las_path).unwrap();
        let mut count = 0usize;
        while let Some(point) = reader.points().next() {
            let point = point.unwrap();
            if count == 0 {
                assert!((point.x - 341_001.5).abs() <= 0.01);
                assert!((point.y - 2_545_002.5).abs() <= 0.01);
            }
            count += 1;
        }
        assert_eq!(count, 2);
    }

    #[test]
    fn default_output_dir_is_input_output() {
        let dir = std::env::temp_dir().join("restore_pcd_by_mgrs_test_default_out");
        let input = dir.join("input");
        fs::create_dir_all(&input).unwrap();
        write_test_pcd(&input.join("50QKL416457.pcd"));

        run(&[
            "--input".to_string(),
            input.to_string_lossy().into_owned(),
            "--threads".to_string(),
            "1".to_string(),
        ])
        .unwrap();

        assert!(input.join("output").join("50QKL416457_restored.las").exists());
    }
}
