//! MGRS 文件名解析：从白犀牛百米块文件名推导 1km 块西南角的绝对 UTM 偏移。
//!
//! 文件名形如 `50QKL416457.pcd`：zone(50) + 纬度带(Q) + 100km 列字母(K) + 行字母(L)
//! + 东偏/北偏数字串（各 3 位，百米块）。解析规则与原 Python 工具一致：
//! 数字串前半为东偏、后半为北偏，各取前两位公里值截断到 1km 块西南角，
//! 再解出该角点的绝对 UTM 坐标作为还原偏移。

/// 1km 块西南角对应的绝对 UTM 偏移与 EPSG 代码。
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) struct MgrsUtmOffset {
    pub epsg: u16,
    pub offset_x: i64,
    pub offset_y: i64,
}

/// MGRS 100km 列字母表（24 个，跳过 I、O）。
const COL_LETTERS: &[u8; 24] = b"ABCDEFGHJKLMNPQRSTUVWXYZ";
/// MGRS 100km 行字母表（20 个，跳过 I、O）。
const ROW_LETTERS: &[u8; 20] = b"ABCDEFGHJKLMNPQRSTUV";
/// 纬度带字母（C..=X，跳过 I、O），与南边界纬度一一对应。
const BAND_LETTERS: &[u8; 20] = b"CDEFGHJKLMNPQRSTUVWX";
/// 各纬度带南边界纬度（度），带 X 为 72°N-84°N 特例不影响南边界。
const BAND_MIN_LAT: [f64; 20] = [
    -80.0, -72.0, -64.0, -56.0, -48.0, -40.0, -32.0, -24.0, -16.0, -8.0, 0.0, 8.0, 16.0, 24.0,
    32.0, 40.0, 48.0, 56.0, 64.0, 72.0,
];

const WGS84_A: f64 = 6_378_137.0;
const WGS84_F: f64 = 1.0 / 298.257_223_563;
const UTM_K0: f64 = 0.9996;
const UTM_FALSE_NORTHING_SOUTH: f64 = 10_000_000.0;
/// 行字母以 2000km 为周期循环。
const ROW_CYCLE: i64 = 2_000_000;

/// 从文件名解析 MGRS 偏移；文件名不含有效 MGRS 块时返回 None。
pub(crate) fn parse_mgrs_offset(filename: &str) -> Option<MgrsUtmOffset> {
    decode(&capture_mgrs(filename)?)
}

struct MgrsCapture {
    zone: u8,
    band: u8,
    col: u8,
    row: u8,
    digits: Vec<u8>,
}

/// 在文件名中从左到右扫描 MGRS 块（等价于正则 `(\d{1,2}[C-X][A-Z]{2}\d+)` 的首个匹配）。
fn capture_mgrs(filename: &str) -> Option<MgrsCapture> {
    let bytes = filename.as_bytes();
    for start in 0..bytes.len() {
        if let Some(cap) = try_capture_at(bytes, start) {
            return Some(cap);
        }
    }
    None
}

fn try_capture_at(bytes: &[u8], start: usize) -> Option<MgrsCapture> {
    // zone 优先取 2 位数字，失败回退 1 位（对应正则 \d{1,2} 的贪婪回溯）
    for zone_len in [2usize, 1] {
        let Some(zone_digits) = bytes.get(start..start + zone_len) else {
            continue;
        };
        if !zone_digits.iter().all(u8::is_ascii_digit) {
            continue;
        }
        let zone: u8 = std::str::from_utf8(zone_digits).ok()?.parse().ok()?;
        if !(1..=60).contains(&zone) {
            continue;
        }

        let band_pos = start + zone_len;
        let band = *bytes.get(band_pos)?;
        let col = *bytes.get(band_pos + 1)?;
        let row = *bytes.get(band_pos + 2)?;
        if !band.is_ascii_uppercase() || !col.is_ascii_uppercase() || !row.is_ascii_uppercase() {
            continue;
        }
        // MGRS 字母表不含 I、O（原 Python 实现经由 mgrs 库解析失败同样跳过）
        if band == b'I' || band == b'O' || col == b'I' || col == b'O' || row == b'I' || row == b'O'
        {
            continue;
        }

        let digits_start = band_pos + 3;
        let mut digits_end = digits_start;
        while bytes.get(digits_end).is_some_and(u8::is_ascii_digit) {
            digits_end += 1;
        }
        if digits_end == digits_start {
            continue;
        }

        return Some(MgrsCapture {
            zone,
            band,
            col,
            row,
            digits: bytes[digits_start..digits_end].to_vec(),
        });
    }
    None
}

fn decode(cap: &MgrsCapture) -> Option<MgrsUtmOffset> {
    let band_idx = BAND_LETTERS.iter().position(|&c| c == cap.band)?;
    let northern = cap.band >= b'N';

    // 数字串前半为东偏、后半为北偏，各取前两位公里值截断到 1km 块西南角
    let digits = &cap.digits;
    if digits.len() < 4 || digits.len() % 2 != 0 {
        return None;
    }
    let half = digits.len() / 2;
    let km_e = parse_km(&digits[..2])?;
    let km_n = parse_km(&digits[half..half + 2])?;

    // 列字母 → 100km 方格西边缘的绝对东偏
    let col_idx = COL_LETTERS.iter().position(|&c| c == cap.col)?;
    let set_start = [0usize, 8, 16][(usize::from(cap.zone) - 1) % 3];
    let col_pos = col_idx.checked_sub(set_start)?;
    if col_pos > 7 {
        return None;
    }
    let easting_base = 100_000 * (i64::from((cap.zone - 1) % 3) + 1 + col_pos as i64);

    // 行字母 → 100km 方格南边缘（模 2000km），用纬度带南边界解循环歧义
    let row_idx = ROW_LETTERS.iter().position(|&c| c == cap.row)?;
    let zone_offset: i64 = if cap.zone % 2 == 0 { 5 } else { 0 };
    let mut northing_base = ((row_idx as i64 - zone_offset + 20) % 20) * 100_000;
    let band_min = band_min_northing(BAND_MIN_LAT[band_idx]);
    while northing_base < band_min {
        northing_base += ROW_CYCLE;
    }

    let epsg = if northern {
        32_600 + u16::from(cap.zone)
    } else {
        32_700 + u16::from(cap.zone)
    };
    Some(MgrsUtmOffset {
        epsg,
        offset_x: easting_base + i64::from(km_e) * 1000,
        offset_y: northing_base + i64::from(km_n) * 1000,
    })
}

fn parse_km(two: &[u8]) -> Option<u16> {
    std::str::from_utf8(two).ok()?.parse::<u16>().ok()
}

/// 纬度带南边界在中央经线处的 UTM 北偏（米）；南半球含 10000km 假北偏。
fn band_min_northing(lat_deg: f64) -> i64 {
    let northing = UTM_K0 * meridian_arc(lat_deg)
        + if lat_deg < 0.0 {
            UTM_FALSE_NORTHING_SOUTH
        } else {
            0.0
        };
    northing.round() as i64
}

/// WGS84 子午线弧长（米），纬度带符号。
fn meridian_arc(lat_deg: f64) -> f64 {
    let e2 = WGS84_F * (2.0 - WGS84_F);
    let e4 = e2 * e2;
    let e6 = e4 * e2;
    let lat = lat_deg.to_radians();
    WGS84_A
        * ((1.0 - e2 / 4.0 - 3.0 * e4 / 64.0 - 5.0 * e6 / 256.0) * lat
            - (3.0 * e2 / 8.0 + 3.0 * e4 / 32.0 + 45.0 * e6 / 1024.0) * (2.0 * lat).sin()
            + (15.0 * e4 / 256.0 + 45.0 * e6 / 1024.0) * (4.0 * lat).sin()
            - (35.0 * e6 / 3072.0) * (6.0 * lat).sin())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decodes_northern_100m_block() {
        // zone 50 偶数区：列 K → 东偏 300km 起；行 L → 500km 起，Q 带(16°N-24°N)
        // 南边界北偏约 1769km，解循环后行基准 = 2500km
        let off = parse_mgrs_offset("50QKL416457.pcd").unwrap();
        assert_eq!(off.epsg, 32650);
        assert_eq!(off.offset_x, 341_000);
        assert_eq!(off.offset_y, 2_545_000);
    }

    #[test]
    fn decodes_southern_100m_block() {
        // zone 34 偶数区：列 F → 东偏 600km 起；行 G → 100km 起，H 带(40°S-32°S)
        // 南边界北偏约 5572km，解循环后行基准 = 6100km
        let off = parse_mgrs_offset("34HFG123456.pcd").unwrap();
        assert_eq!(off.epsg, 32734);
        assert_eq!(off.offset_x, 612_000);
        assert_eq!(off.offset_y, 6_145_000);
    }

    #[test]
    fn finds_mgrs_embedded_in_filename() {
        let off = parse_mgrs_offset("tile_50QKL416457_meta.pcd").unwrap();
        assert_eq!(off.epsg, 32650);
    }

    #[test]
    fn rejects_invalid_names() {
        assert!(parse_mgrs_offset("plain_name.pcd").is_none());
        assert!(parse_mgrs_offset("50QKI416457.pcd").is_none()); // I 不在 MGRS 字母表
        assert!(parse_mgrs_offset("50QKL41.pcd").is_none()); // 数字串不足
        assert!(parse_mgrs_offset("50QKL416.pcd").is_none()); // 奇数位数字串
        assert!(parse_mgrs_offset("50qkl416457.pcd").is_none()); // 小写
        assert!(parse_mgrs_offset("61QKL416457.pcd").is_none()); // zone 越界
    }

    #[test]
    fn truncates_to_1km_block_regardless_of_precision() {
        // 10m（8 位）与百米（6 位）精度截断到同一 1km 块
        let coarse = parse_mgrs_offset("50QKL416457.pcd").unwrap();
        let fine = parse_mgrs_offset("50QKL41644575.pcd").unwrap();
        assert_eq!(coarse, fine);
    }

    #[test]
    fn roundtrips_against_forward_utm() {
        // 用独立的 UTM 正算 + MGRS 编码方向做闭环校验：
        // 任意点的百米块文件名解码出的 1km 偏移应包含该点
        let cases = [
            (22.123_f64, 117.456_f64, 50_u8), // Q 带
            (-35.31, 149.15, 55),              // H 带（南半球，偶数区）
            (63.85, -19.06, 27),               // W 带
        ];
        for (lat, lon, zone) in cases {
            let (e, n) = forward_utm(lat, lon, zone);
            let band = band_letter(lat);
            let col = col_letter(zone, e);
            let row = row_letter(zone, n);
            let name = format!(
                "{}{}{}{}{:03}{:03}.pcd",
                zone,
                band,
                col,
                row,
                (e % 100_000.0) as u32 / 100,
                (n % 100_000.0) as u32 / 100
            );
            let off = parse_mgrs_offset(&name)
                .unwrap_or_else(|| panic!("解析失败: {name} (e={e}, n={n})"));
            assert_eq!(
                off.offset_x,
                (e / 1000.0).floor() as i64 * 1000,
                "东偏不匹配: {name}"
            );
            assert_eq!(
                off.offset_y,
                (n / 1000.0).floor() as i64 * 1000,
                "北偏不匹配: {name}"
            );
        }
    }

    // ---- 测试专用的编码方向实现（与解码路径独立） ----

    fn forward_utm(lat_deg: f64, lon_deg: f64, zone: u8) -> (f64, f64) {
        let e2 = WGS84_F * (2.0 - WGS84_F);
        let ep2 = e2 / (1.0 - e2);
        let lat = lat_deg.to_radians();
        let lon = lon_deg.to_radians();
        let lon_origin = ((f64::from(zone) - 1.0) * 6.0 - 180.0 + 3.0).to_radians();
        let sin_lat = lat.sin();
        let cos_lat = lat.cos();
        let tan_lat = lat.tan();
        let n = WGS84_A / (1.0 - e2 * sin_lat * sin_lat).sqrt();
        let t = tan_lat * tan_lat;
        let c = ep2 * cos_lat * cos_lat;
        let a = cos_lat * (lon - lon_origin);
        let m = meridian_arc(lat_deg);
        let easting = 500_000.0
            + UTM_K0
                * n
                * (a + (1.0 - t + c) * a.powi(3) / 6.0
                    + (5.0 - 18.0 * t + t * t + 72.0 * c - 58.0 * ep2) * a.powi(5) / 120.0);
        let mut northing =
            UTM_K0 * (m + n * tan_lat * (a.powi(2) / 2.0 + (5.0 - t + 9.0 * c + 4.0 * c * c) * a.powi(4) / 24.0
                + (61.0 - 58.0 * t + t * t + 600.0 * c - 330.0 * ep2) * a.powi(6) / 720.0));
        if lat_deg < 0.0 {
            northing += UTM_FALSE_NORTHING_SOUTH;
        }
        (easting, northing)
    }

    fn band_letter(lat: f64) -> char {
        let idx = ((lat + 80.0) / 8.0).floor().max(0.0).min(19.0) as usize;
        BAND_LETTERS[idx] as char
    }

    fn col_letter(zone: u8, easting: f64) -> char {
        let set_start = [0usize, 8, 16][(usize::from(zone) - 1) % 3];
        let base_hundreds = usize::from((zone - 1) % 3) + 1;
        let pos = (easting / 100_000.0) as usize - base_hundreds;
        COL_LETTERS[set_start + pos] as char
    }

    fn row_letter(zone: u8, northing: f64) -> char {
        let offset = if zone % 2 == 0 { 5usize } else { 0 };
        let hundreds = (northing / 100_000.0) as usize;
        ROW_LETTERS[(hundreds % 20 + offset) % 20] as char
    }
}
