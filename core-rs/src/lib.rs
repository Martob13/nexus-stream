#[no_mangle]
pub extern "C" fn calculate_energy_rms(samples: *const f64, length: usize) -> f64 {
    if samples.is_null() || length == 0 {
        return 0.0;
    }
    let slice = unsafe { std::slice::from_raw_parts(samples, length) };
    let sum_sq: f64 = slice.iter().map(|&x| x * x).sum();
    (sum_sq / length as f64).sqrt()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rms_calculation() {
        let data = vec![2.0, -2.0, 2.0, -2.0];
        let rms = calculate_energy_rms(data.as_ptr(), data.len());
        assert!((rms - 2.0).abs() < 1e-6);
    }
}
