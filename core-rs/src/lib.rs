use std::slice;

pub const MAGIC_0: u8 = 0x55;
pub const MAGIC_1: u8 = 0xAA;
pub const HEADER_SIZE: usize = 16;

#[repr(C)]
#[derive(Debug, Default, Clone, Copy, PartialEq)]
pub struct SignalMetrics {
    pub rms: f64,
    pub peak: f64,
}

/// Decodifica una trama binaria y calcula las métricas RMS y pico.
///
/// # Safety
///
/// - `raw_frame_ptr` debe apuntar a un bloque contiguo y legible de `frame_len` bytes.
/// - `out_metrics` debe ser un puntero válido y alineado hacia una estructura `SignalMetrics` escribible.
/// - El llamador garantiza que ambos punteros no sean nulos durante la ejecución.
#[no_mangle]
pub unsafe extern "C" fn parse_and_compute_metrics(
    raw_frame_ptr: *const u8,
    frame_len: usize,
    out_metrics: *mut SignalMetrics,
) -> i32 {
    if raw_frame_ptr.is_null() || out_metrics.is_null() {
        return -1;
    }

    if frame_len < HEADER_SIZE {
        return -2;
    }

    let frame = slice::from_raw_parts(raw_frame_ptr, frame_len);

    if frame[0] != MAGIC_0 || frame[1] != MAGIC_1 {
        return -3;
    }

    let sample_count = u16::from_le_bytes([frame[10], frame[11]]) as usize;
    let expected_len = HEADER_SIZE + (sample_count * 8);
    if frame_len != expected_len {
        return -4;
    }

    if sample_count == 0 {
        (*out_metrics).rms = 0.0;
        (*out_metrics).peak = 0.0;
        return 0;
    }

    let samples_byte_slice = &frame[HEADER_SIZE..expected_len];

    if !(samples_byte_slice.as_ptr() as usize).is_multiple_of(std::mem::align_of::<f64>()) {
        let (rms, peak) = compute_unaligned(samples_byte_slice, sample_count);
        (*out_metrics).rms = rms;
        (*out_metrics).peak = peak;
        return 0;
    }

    let samples: &[f64] = slice::from_raw_parts(
        samples_byte_slice.as_ptr() as *const f64,
        sample_count,
    );

    let (rms, peak) = compute_simd_unrolled(samples);
    (*out_metrics).rms = rms;
    (*out_metrics).peak = peak;

    0
}

#[inline(always)]
fn compute_simd_unrolled(samples: &[f64]) -> (f64, f64) {
    let mut sum_sq_0 = 0.0;
    let mut sum_sq_1 = 0.0;
    let mut sum_sq_2 = 0.0;
    let mut sum_sq_3 = 0.0;

    let mut peak_0: f64 = 0.0;
    let mut peak_1: f64 = 0.0;
    let mut peak_2: f64 = 0.0;
    let mut peak_3: f64 = 0.0;

    let chunks = samples.chunks_exact(4);
    let remainder = chunks.remainder();

    for chunk in chunks {
        let v0 = chunk[0];
        let v1 = chunk[1];
        let v2 = chunk[2];
        let v3 = chunk[3];

        sum_sq_0 += v0 * v0;
        sum_sq_1 += v1 * v1;
        sum_sq_2 += v2 * v2;
        sum_sq_3 += v3 * v3;

        let a0 = v0.abs();
        let a1 = v1.abs();
        let a2 = v2.abs();
        let a3 = v3.abs();

        if a0 > peak_0 { peak_0 = a0; }
        if a1 > peak_1 { peak_1 = a1; }
        if a2 > peak_2 { peak_2 = a2; }
        if a3 > peak_3 { peak_3 = a3; }
    }

    let mut sum_sq = sum_sq_0 + sum_sq_1 + sum_sq_2 + sum_sq_3;
    let mut peak = peak_0.max(peak_1).max(peak_2).max(peak_3);

    for &v in remainder {
        sum_sq += v * v;
        let a = v.abs();
        if a > peak {
            peak = a;
        }
    }

    let rms = (sum_sq / samples.len() as f64).sqrt();
    (rms, peak)
}

fn compute_unaligned(bytes: &[u8], count: usize) -> (f64, f64) {
    let mut sum_sq = 0.0;
    let mut peak: f64 = 0.0;

    for i in 0..count {
        let offset = i * 8;
        let val = f64::from_le_bytes(bytes[offset..offset + 8].try_into().unwrap());
        sum_sq += val * val;
        let a = val.abs();
        if a > peak {
            peak = a;
        }
    }

    ((sum_sq / count as f64).sqrt(), peak)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_binary_frame_processing() {
        let mut frame = [0u8; HEADER_SIZE + (4 * 8)];
        frame[0] = MAGIC_0;
        frame[1] = MAGIC_1;
        frame[10] = 4;
        frame[11] = 0;

        let samples: [f64; 4] = [1.0, -2.0, 3.0, -4.0];
        for (i, &s) in samples.iter().enumerate() {
            let b = s.to_le_bytes();
            frame[HEADER_SIZE + (i * 8)..HEADER_SIZE + ((i + 1) * 8)].copy_from_slice(&b);
        }

        let mut metrics = SignalMetrics::default();
        let res = unsafe { parse_and_compute_metrics(frame.as_ptr(), frame.len(), &mut metrics) };

        assert_eq!(res, 0);
        assert!((metrics.peak - 4.0).abs() < 1e-6);
        let expected_rms = ((1.0 + 4.0 + 9.0 + 16.0) / 4.0_f64).sqrt();
        assert!((metrics.rms - expected_rms).abs() < 1e-6);
    }

    #[test]
    fn test_invalid_magic_rejection() {
        let frame = [0xFF, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
        let mut metrics = SignalMetrics::default();
        let res = unsafe { parse_and_compute_metrics(frame.as_ptr(), frame.len(), &mut metrics) };
        assert_eq!(res, -3);
    }

    #[test]
    fn test_short_frame_rejection() {
        let frame = [MAGIC_0, MAGIC_1];
        let mut metrics = SignalMetrics::default();
        let res = unsafe { parse_and_compute_metrics(frame.as_ptr(), frame.len(), &mut metrics) };
        assert_eq!(res, -2);
    }
}
