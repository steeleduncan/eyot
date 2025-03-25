/*
  OpenCL Eyot runtime definitions

  Eyot pipeline code, this merges multiple workers together
 */

#include "eyot-runtime-cpu.h"
#include <pthread.h>

/*
  A naive pipeline

  This uses(wastes) a CPU thread copying from one to the next
  A refactor would make these push directly
 */
typedef struct EyNaivePipeline {
    // lhs is the first worker in the line, rhs the second
    EyWorker *lhs, *rhs;

    pthread_t thread;
    pthread_mutex_t mutex;

    int underway_count;
} EyNaivePipeline;

static void ey_naive_pipeline_entry_point(EyNaivePipeline *pipeline) {
    // TODO we should never be draining, we should pass them one by one
    // however that might be better done when the pipelines are structured as push pipes
    EyVector *results = pipeline->lhs->drain(pipeline->lhs);
    pipeline->rhs->send(pipeline->rhs, results);
}

static void *thread_entry(void *ctx) {
    ey_naive_pipeline_entry_point(ctx);
    return 0;
}

static void ey_pipeline_send(EyWorker *wrkr, EyVector *values) {
    EyNaivePipeline *pipeline = (EyNaivePipeline *)wrkr->ctx;
    pthread_mutex_lock(&pipeline->mutex);
    pipeline->underway_count += ey_vector_length(0, values);
    pthread_mutex_unlock(&pipeline->mutex);
    pipeline->lhs->send(pipeline->lhs, values);
}

static void ey_pipeline_receive(EyWorker *wrkr, void *value) {
    EyNaivePipeline *pipeline = (EyNaivePipeline *)wrkr->ctx;
    pipeline->rhs->receive(pipeline->rhs, value);
    pthread_mutex_lock(&pipeline->mutex);
    pipeline->underway_count -= 1;
    pthread_mutex_unlock(&pipeline->mutex);
}

static EyVector *ey_pipeline_drain(EyWorker *wrkr) {
    EyNaivePipeline *pipeline = (EyNaivePipeline *)wrkr->ctx;

    pthread_mutex_lock(&pipeline->mutex);
    const int required_count = pipeline->underway_count;
    pthread_mutex_unlock(&pipeline->mutex);

    EyVector *results = 0;
    if (wrkr->output_size) {
        results = ey_vector_create(0, wrkr->output_size);
        ey_vector_resize(0, results, required_count);
    }

    for (int i = 0; i < required_count; i += 1) {
        if (results) {
            ey_pipeline_receive(wrkr, ey_vector_access(0, results, i));
        } else {
            ey_pipeline_receive(wrkr, 0);
        }
    }

    return results;
}

/*
  Pipeline
 */
EyWorker *ey_worker_create_pipeline(EyWorker *lhs, EyWorker *rhs) {
    EyGCRegion *gc = ey_runtime_gc(0);

    EyNaivePipeline *pipeline = ey_runtime_gc_alloc(gc, sizeof(EyNaivePipeline), 0);
    if (!pipeline) {
        ey_runtime_panic("ey_worker_create_pipeline", "failed to allocate pipeline");
    }
    *pipeline = (EyNaivePipeline){
        .lhs = lhs,
        .rhs = rhs,
        .underway_count = 0,
    };
    pthread_mutex_init(&pipeline->mutex, 0);

    pthread_create(&pipeline->thread, 0, thread_entry, pipeline);

    EyWorker *w = ey_runtime_gc_alloc(gc, sizeof(EyWorker), 0);
    if (!w) {
        ey_runtime_panic("ey_worker_create_pipeline", "failed to allocate worker");
    }
    *w = (EyWorker){
        .send = ey_pipeline_send,
        .receive = ey_pipeline_receive,
        .drain = ey_pipeline_drain,
        .ctx = pipeline,
        .output_size = rhs->output_size,
    };
    return w;
}
