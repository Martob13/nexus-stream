#ifndef CORE_PARSER_H
#define CORE_PARSER_H

#include <stddef.h>
#include <stdint.h>

#define FRAME_MAGIC_0 0x55
#define FRAME_MAGIC_1 0xAA
#define FRAME_HEADER_SIZE 16

typedef struct {
    double rms;
    double peak;
} SignalMetrics;

#ifdef __cplusplus
extern "C" {
#endif

int32_t parse_and_compute_metrics(
    const uint8_t *raw_frame_ptr,
    size_t frame_len,
    SignalMetrics *out_metrics
);

#ifdef __cplusplus
}
#endif

#endif /* CORE_PARSER_H */
