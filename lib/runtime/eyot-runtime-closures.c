/*
  Eyot closure support
 */

#include "eyot-runtime-cpu.h"

#include <string.h>

EyClosure ey_closure_create(int fid, void **args) {
    EyClosure c = ey_runtime_gc_alloc(ey_runtime_gc(0), ey_generated_closure_size(fid), 0);
    *(int *)c = fid;

    const int ac = ey_generated_arg_count(fid);
    for (int i = 0; i < ac; i += 1) {
        void *arg = args[i];
        if (arg) {
            void *dest = ey_closure_arg_pointer(c, i);
            ey_closure_set_arg_exists(c, i, k_true);
            memcpy(dest, arg, ey_generated_closure_arg_size(fid, i));
        } else {
            ey_closure_set_arg_exists(c, i, k_false);
        }
    }

    return c;
}
