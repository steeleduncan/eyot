/*
  Eyot runtime vectors support
 */

#include "eyot-runtime-cpu.h"

#include <string.h>

typedef struct EyVector {
    int length;
    int unit_size;
    void *ptr;
} EyVector;

EyVector *ey_vector_create(EyExecutionContext *ey_execution_context, int unit_size) {
    EyVector *vec = ey_runtime_gc_alloc(ey_runtime_gc(ey_execution_context), sizeof(EyVector), 0);
    if (vec == 0) {
        ey_runtime_panic("ey_vector_create", "unable to allocate");
    }
    vec->length = 0;
    vec->unit_size = unit_size;
    vec->ptr = 0;
    return vec;
}

void ey_vector_resize(EyExecutionContext *ey_execution_context, EyVector *vec, int new_length) {
    vec->length = new_length;
    if (vec->length == 0) {
        if (vec->ptr) {
            vec->ptr = 0;
        }
    } else {
        if (vec->ptr) {
            vec->ptr = ey_runtime_gc_realloc(ey_runtime_gc(ey_execution_context), vec->ptr,
                                             vec->unit_size * vec->length);
            if (!vec->ptr) {
                ey_runtime_panic("ey_vector_resize", "reallocation failed");
            }
        } else {
            vec->ptr = ey_runtime_gc_alloc(ey_runtime_gc(ey_execution_context),
                                           vec->unit_size * vec->length, 0);
            if (!vec->ptr) {
                ey_runtime_panic("ey_vector_resize", "allocation failed");
            }
        }
    }
}

void ey_vector_erase(EyExecutionContext *ey_execution_context, EyVector *vec, EyInteger start,
                     EyInteger count) {
    if (count == 0) {
        return;
    }

    if (start + count > vec->length) {
        ey_runtime_panic("ey_vector_erase", "deleting out of range of vector");
    }

    for (EyInteger i = start; i < vec->length - count; i += 1) {
        memcpy(ey_vector_access(ey_execution_context, vec, i),
               ey_vector_access(ey_execution_context, vec, i + count), vec->unit_size);
    }

    ey_vector_resize(ey_execution_context, vec, vec->length - count);
}

void *ey_vector_get_ptr(EyExecutionContext *ey_execution_context __attribute__((unused)),
                        EyVector *vec) {
    return vec->ptr;
}

void *ey_vector_access(EyExecutionContext *ey_execution_context __attribute__((unused)),
                       EyVector *vec, int index) {
    if (index < 0) {
        ey_runtime_panic("ey_vector_access", "index out of range (-ve)");
    }
    if (index >= vec->length) {
        ey_runtime_panic("ey_vector_access", "index out of range (+ve)");
    }

    return (void *)((uint8_t *)vec->ptr + index * vec->unit_size);
}

int ey_vector_length(EyExecutionContext *ey_execution_context __attribute__((unused)),
                     const EyVector *vec) {
    return vec->length;
}

void ey_vector_append(EyExecutionContext *ey_execution_context, EyVector *vec,
                      const void *new_element) {
    const int new_size = ey_vector_length(ey_execution_context, vec) + 1;
    ey_vector_resize(ey_execution_context, vec, new_size);
    if (new_element) {
        memcpy(ey_vector_access(ey_execution_context, vec, new_size - 1), new_element,
               vec->unit_size);
    }
}

void ey_vector_append_vector(EyExecutionContext *ey_execution_context, EyVector *vec,
                             EyVector *new_elements) {
    if (vec->unit_size != new_elements->unit_size) {
        ey_runtime_panic("ey_vector_append_vector",
                         "cannot append a vector of different pitch size");
    }

    const int old_size = ey_vector_length(ey_execution_context, vec),
              incoming_size = ey_vector_length(ey_execution_context, new_elements),
              new_size = old_size + incoming_size;

    if (incoming_size == 0) {
        return;
    }

    ey_vector_resize(ey_execution_context, vec, new_size);
    memcpy(ey_vector_access(ey_execution_context, vec, old_size),
           ey_vector_access(ey_execution_context, new_elements, 0), vec->unit_size * incoming_size);
}

EyVector *ey_runtime_range(EyExecutionContext *ey_execution_context, EyInteger start, EyInteger end,
                           EyInteger step) {
    EyVector *r = ey_vector_create(ey_execution_context, sizeof(EyInteger));

    if (step == 0) {
        return r;
    }

    EyInteger val = start;
    if (step < 0) {
        if (end > start) {
            return r;
        }

        while (val > end) {
            ey_vector_append(ey_execution_context, r, &val);
            val += step;
        }
    } else {
        if (end < start) {
            return r;
        }

        while (val < end) {
            ey_vector_append(ey_execution_context, r, &val);
            val += step;
        }
    }

    return r;
}
