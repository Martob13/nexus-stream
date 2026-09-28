#ifndef CORE_PARSER_H
#define CORE_PARSER_H

#include <stdint.h>
#include <stddef.h>

typedef struct {
    double rms;
    double peak;
    size_t sample_count;
} SignalMetrics;

int32_t parse_and_compute_metrics(
    const uint8_t *raw_frame_ptr,
    size_t frame_len,
    SignalMetrics *out_metrics
);

#endif // CORE_PARSER_H
