//! 共享时间转换：epoch 秒 ↔ ISO8601。
//!
//! calendar/events 等 node 命令的 NSDate ↔ ISO 字段、startISO/endISO 解析统一走这里。
//! Howard Hinnant 的 civil_from_days / days_from_civil 互逆算法（无 chrono 依赖）。

/// epoch 秒（f64，UTC）→ "YYYY-MM-DDTHH:MM:SSZ"。
pub fn epoch_to_iso(secs: f64) -> String {
    let total = secs as i64;
    let s = total.rem_euclid(60);
    let m = total.div_euclid(60).rem_euclid(60);
    let h = total.div_euclid(3600).rem_euclid(24);
    let days = total.div_euclid(86400);
    let (y, mo, d) = civil_from_days(days);
    format!("{y:04}-{mo:02}-{d:02}T{h:02}:{m:02}:{s:02}Z")
}

/// days since unix epoch → (year, month, day)（Hinnant civil_from_days）。
pub fn civil_from_days(z: i64) -> (i64, i64, i64) {
    let z = z + 719468;
    let era = (if z >= 0 { z } else { z - 146096 }) / 146097;
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    (if m <= 2 { y + 1 } else { y }, m, d)
}

/// (year, month, day) → days since unix epoch（Hinnant days_from_civil，与 civil_from_days 互逆）。
pub fn days_from_civil(y: i64, m: i64, d: i64) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = (if y >= 0 { y } else { y - 399 }) / 400;
    let yoe = y - era * 400;
    let doy = (153 * (if m > 2 { m - 3 } else { m + 9 }) + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

/// ISO8601 字符串 → epoch 秒（f64, UTC）。支持：
/// "YYYY-MM-DD" / "YYYY-MM-DDTHH:MM:SS" / "...SSZ" / "...SS.sssZ" / "...SS±HH:MM"。
/// 无法解析返 None（调用方降级到默认值）。
pub fn iso_to_epoch(s: &str) -> Option<f64> {
    let s = s.trim();
    // 日期/时间分隔（T 或空格）
    let (date_part, time_part) = match s.split_once(['T', ' ']) {
        Some((d, t)) => (d, Some(t)),
        None => (s, None),
    };
    let mut dit = date_part.split('-');
    let y = dit.next()?.parse::<i64>().ok()?;
    let mo = dit.next()?.parse::<i64>().ok()?;
    let d = dit.next()?.parse::<i64>().ok()?;
    let mut secs = days_from_civil(y, mo, d) as f64 * 86400.0;
    if let Some(tp) = time_part {
        let (main, tz) = split_tz(tp);
        let mut tit = main.split(':');
        let h: f64 = tit.next().unwrap_or("0").parse().ok()?;
        let mi: f64 = tit.next().unwrap_or("0").parse().ok()?;
        // 秒位可能带 ".sss" 小数
        let se_raw = tit.next().unwrap_or("0");
        let se: f64 = se_raw.parse().ok()?;
        secs += h * 3600.0 + mi * 60.0 + se;
        if let Some(off_secs) = parse_tz_offset(tz) {
            secs -= off_secs; // 本地偏移 → UTC：减去偏移
        }
    }
    Some(secs)
}

/// 从时间部分拆出主体与时区后缀（"Z" / "±HH:MM" / 无）。
fn split_tz(tp: &str) -> (&str, Option<&str>) {
    if let Some(stripped) = tp.strip_suffix('Z').or_else(|| tp.strip_suffix('z')) {
        return (stripped, None);
    }
    // 找时区符号（'+'/'-'），秒的小数点 '.' 不是时区
    if let Some(pos) = tp.rfind(['+', '-']) {
        if pos > 0 {
            // '+'/'-' 后形如 HH:MM
            let cand = &tp[pos..];
            if cand.len() >= 3 && (cand.as_bytes()[3] == b':' || cand.len() == 3) {
                return (&tp[..pos], Some(cand));
            }
        }
    }
    (tp, None)
}

/// "±HH:MM" → 秒（None 表示 UTC/无偏移）。
fn parse_tz_offset(tz: Option<&str>) -> Option<f64> {
    let tz = tz?;
    let (sign, rest) = tz.split_at(1);
    let mut it = rest.split(':');
    let h: f64 = it.next()?.parse().ok()?;
    let m: f64 = it.next().unwrap_or("0").parse().ok()?;
    let mag = h * 3600.0 + m * 60.0;
    Some(if sign == "-" { -mag } else { mag })
}
