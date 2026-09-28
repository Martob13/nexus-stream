use std::slice;

#[repr(C)]
#[derive(Debug, PartialEq)]
pub struct SignalMetrics {
    pub rms: f64,
    pub peak: f64,
    pub sample_count: usize,
}

/// Binary frame header layout:
/// [0..2]   Magic: 0xAA55
/// [2..10]  Timestamp (u64 little-endian)
/// [10..12] Sample count (u16 little-endian)
/// [12..]   Samples (f64 little-endian array)
pub const HEADER_SIZE: usize = 12;
pub const MAGIC_BYTES: [u8; 2] = [0x55, 0xAA];

#[no_mangle]
pub unsafe extern "C" fn parse_and_compute_metrics(
    raw_frame_ptr: *const u8,
    frame_len: usize,
    out_metrics: *mut SignalMetrics,
) -> i32 {
    if raw_frame_ptr.is_null() || out_metrics.is_null() {
        return -1; // Null pointer error
    }

    if frame_len < HEADER_SIZE {
        return -2; // Frame smaller than header
    }

    let buffer = slice::from_raw_parts(raw_frame_ptr, frame_len);

    // Validate magic bytes
    if buffer[0] != MAGIC_BYTES[0] || buffer[1] != MAGIC_BYTES[1] {
        return -3; // Invalid frame magic
    }

    let sample_count = u16::from_le_bytes([buffer[10], buffer[11]]) as usize;
    let expected_payload_len = sample_count * std::mem::size_of::<f64>();

    if frame_len != HEADER_SIZE + expected_payload_len {
        return -4; // Length mismatch
    }

    if sample_count == 0 {
        (*out_metrics).rms = 0.0;
        (*out_metrics).peak = 0.0;
        (*out_metrics).sample_count = 0;
        return 0;
    }

    // Zero-copy cast into &[f64] with alignment safety check
    let samples_byte_slice = &buffer[HEADER_SIZE..];
    if (samples_byte_slice.as_ptr() as usize) % std::mem::align_of::<f64>() != 0 {
        // Fallback: decode unaligned f64 values without heap allocations
        return compute_unaligned(samples_byte_slice, sample_count, out_metrics);
    }

    let samples = slice::from_raw_parts(
        samples_byte_slice.as_ptr() as *const f64,
        sample_count,
    );

    compute_simd_unrolled(samples, out_metrics);
    0
}

#[inline(always)]
unsafe fn compute_unaligned(bytes: &[u8], count: usize, out_metrics: *mut SignalMetrics) -> i32 {
    let mut sum_sq = 0.0;
    let mut max_peak = 0.0;

    for i in 0..count {
        let offset = i * 8;
        let val = f64::from_le_bytes(bytes[offset..offset + 8].try_into().unwrap());
        sum_sq += val * val;
        let abs = val.abs();
        if abs > max_peak {
            max_peak = abs;
        }
    }

    (*out_metrics).rms = (sum_sq / count as f64).sqrt();
    (*out_metrics).peak = max_peak;
    (*out_metrics).sample_count = count;
    0
}

#[inline(always)]
unsafe fn compute_simd_unrolled(samples: &[f64], out_metrics: *mut SignalMetrics) {
    let mut sum_sq = 0.0;
    let mut max_peak = 0.0;

    let chunks = samples.chunks_exact(4);
    let remainder = chunks.remainder();

    for chunk in chunks {
        let s0 = chunk[0] * chunk[0];
        let s1 = chunk[1] * chunk[1];
        let s2 = chunk[2] * chunk[2];
        let s3 = chunk[3] * chunk[3];
        sum_sq += s0 + s1 + s2 + s3;

        let a0 = chunk[0].abs();
        let a1 = chunk[1].abs();
        let a2 = chunk[2].abs();
        let a3 = chunk[3].abs();
        if a0 > max_peak { max_peak = a0; }
        if a1 > max_peak { max_peak = a1; }
        if a2 > max_peak { max_peak = a2; }
        if a3 > max_peak { max_peak = a3; }
    }

    for &val in remainder {
        sum_sq += val * val;
        let abs = val.abs();
        if abs > max_peak {
            max_peak = abs;
        }
    }

    (*out_metrics).rms = (sum_sq / samples.len() as f64).sqrt();
    (*out_metrics).peak = max_peak;
    (*out_metrics).sample_count = samples.len();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_binary_frame_processing() {
        let mut frame = Vec::new();
        frame.extend_from_slice(&MAGIC_BYTES); // 0x55, 0xAA
        frame.extend_from_slice(&1695900000u64.to_le_bytes()); // Timestamp
        let sample_count = 5u16;
        frame.extend_from_slice(&sample_count.to_le_bytes());

        let raw_samples = vec![1.0f64, -2.0f64, 3.0f64, -4.0f64, 5.0f64];
        for s in raw_samples {
            frame.extend_from_slice(&s.to_le_bytes());
        }

        let mut metrics = SignalMetrics {
            rms: 0.0,
            peak: 0.0,
            sample_count: 0,
        };

        let result = unsafe {
            parse_and_compute_metrics(frame.as_ptr(), frame.len(), &mut metrics)
        };

        assert_eq!(result, 0);
        assert_eq!(metrics.sample_count, 5);
        assert_eq!(metrics.peak, 5.0);
        assert!((metrics.rms - 3.3166247).abs() < 1e-5);
    }

    #[test]
    fn test_invalid_magic_rejection() {
        let bad_frame = vec![0x00, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
        let mut metrics = SignalMetrics { rms: 0.0, peak: 0.0, sample_count: 0 };
        let result = unsafe {
            parse_and_compute_metrics(bad_frame.as_ptr(), bad_frame.len(), &mut metrics)
        };
        assert_eq!(result, -3);
    }
}
