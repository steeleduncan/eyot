/*
  OpenCL Eyot runtime definitions

  Eyot pipeline code
 */

#include "eyot-runtime-cpu.h"
#include "eyot-runtime-pipe.h"

#include <string.h>
#include <pthread.h>

/*
  A Cpu worker

  This computes within a background thread
 */
typedef struct EyCpuWorker {
    EyWorkerFunction fn;
    EyPipe *input_pipe, *output_pipe;
    int input_size, output_size;
    pthread_t thread;
    void *ctx;

    /*
      How many have been sent and not received back
     */
    int underway_count;

    pthread_mutex_t mutex;
} EyCpuWorker;
void ey_worker_entry_point(EyCpuWorker *w);

static void *thread_entry(void *ctx) {
    ey_worker_entry_point(ctx);
    return 0;
}

static void ey_worker_send(EyWorker *wrkr, EyVector *values) {
    EyCpuWorker *w = wrkr->ctx;

    const int l = ey_vector_length(0, values);

    pthread_mutex_lock(&w->mutex);
    w->underway_count += l;
    pthread_mutex_unlock(&w->mutex);

    for (int i = 0; i < l; i += 1) {
        ey_pipe_send(w->input_pipe, ey_vector_access(0, values, i));
    }
}

static void ey_worker_receive(EyWorker *wrkr, void *value) {
    EyCpuWorker *w = wrkr->ctx;

    if (ey_pipe_receive(w->output_pipe, value)) {
        pthread_mutex_lock(&w->mutex);
        w->underway_count -= 1;
        pthread_mutex_unlock(&w->mutex);
    } else {
        ey_runtime_panic("ey_worker_receive", "failed to receive");
    }
}

static EyVector *ey_worker_drain(EyWorker *wrkr) {
    EyCpuWorker *w = wrkr->ctx;

    pthread_mutex_lock(&w->mutex);
    const int required_count = w->underway_count;
    pthread_mutex_unlock(&w->mutex);

    EyVector *results = 0;

    if (w->output_size) {
        results = ey_vector_create(0, w->output_size);
        ey_vector_resize(0, results, required_count);
    }

    char temp;

    for (int i = 0; i < required_count; i += 1) {
        void *ptr;
        if (w->output_size) {
            ptr = ey_vector_access(0, results, i);
        } else {
            ptr = &temp;
        }
        ey_worker_receive(wrkr, ptr);
    }

    return results;
}

void ey_worker_entry_point(EyCpuWorker *w) {
    // should use malloc?
    void *input = ey_runtime_manual_alloc(w->input_size);
    if (!input) {
        ey_runtime_panic("ey_worker_entry_point", "failed to allocate input");
    }

    void *output = 0;
    if (w->output_size) {
        output = ey_runtime_manual_alloc(w->output_size);
        if (!output) {
            ey_runtime_panic("ey_worker_entry_point", "failed to allocate ouput");
        }
    }

    // currently only non-null for GPU code
    EyExecutionContext *ectx = 0;

    while (ey_pipe_receive(w->input_pipe, input)) {
        w->fn(ectx, input, output, w->ctx);
        if (output) {
            ey_pipe_send(w->output_pipe, output);
        } else {
            char c = 0;
            ey_pipe_send(w->output_pipe, &c);
        }
    }
    if (output) {
        ey_pipe_close(w->output_pipe);
    }

    ey_runtime_gc_forget_root_object(ey_runtime_gc(ectx), w);

    ey_runtime_manual_free(input);
    if (output) {
        ey_runtime_manual_free(output);
    }
}

static void finalise_cpu_worker(void *obj) {
    EyWorker *w = obj;
    EyCpuWorker *wrkr = w->ctx;
    ey_pipe_close(wrkr->input_pipe);
}

EyWorker *ey_worker_create_cpu(EyWorkerFunction fn, int input_size, int output_size, void *raw_ctx,
                               int ctx_size) {
    /*
      We keep a copy for safety
      Nothing should be in the context that is too big to fit on the stack as function args
    */
    void *ctx = 0;
    if (raw_ctx) {
        ctx = ey_runtime_gc_alloc(ey_runtime_gc(0), ctx_size, 0);
        memcpy(ctx, raw_ctx, ctx_size);
    }

    EyCpuWorker *wrkr = ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(EyCpuWorker), 0);
    if (!wrkr) {
        ey_runtime_panic("ey_worker_create_cpu", "failed to allocate cpu worker");
    }
    *wrkr = (EyCpuWorker){
        .fn = fn,
        .input_size = input_size,
        .output_size = output_size,
        .input_pipe = ey_pipe_create(input_size),
        .output_pipe = 0,
        .ctx = ctx,
    };
    pthread_mutex_init(&wrkr->mutex, 0);

    // pin the cpu worker, it can
    ey_runtime_gc_remember_root_object(ey_runtime_gc(0), wrkr);

    if (output_size) {
        wrkr->output_pipe = ey_pipe_create(output_size);
    } else {
        // this is the null output case
        wrkr->output_pipe = ey_pipe_create(1);
    }
    pthread_create(&wrkr->thread, 0, thread_entry, wrkr);

    EyWorker *w = ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(EyWorker), finalise_cpu_worker);
    if (!w) {
        ey_runtime_panic("ey_worker_create_cpu", "failed to allocate worker");
    }
    *w = (EyWorker){
        .send = ey_worker_send,
        .receive = ey_worker_receive,
        .drain = ey_worker_drain,
        .output_size = output_size,
        .ctx = wrkr,
    };
    return w;
}
