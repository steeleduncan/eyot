#include <stdio.h>
#include "eyot-runtime-cpu.h"

EyString ey_stdlib_read_text_file(EyExecutionContext *ctx, EyString path) {
    const char *cpath = ey_runtime_string_create_c_string(path);
    FILE *fh = fopen(cpath, "rb");
    ey_runtime_manual_free(cpath);
    if (!fh) {
        // hardly elegant error handling
        ey_print("No file found at '%s'\n", cpath);
        return ey_runtime_string_create_literal(ctx, "");
    }

    fseek(fh, 0, SEEK_END);
    const int file_size = ftell(fh);
    fseek(fh, 0, SEEK_SET);

    if (file_size == 0) {
        fclose(fh);
        return ey_runtime_string_create_literal(ctx, "");
    }

    void *blk = ey_runtime_manual_alloc(file_size + 1);
    ((unsigned char *)blk)[file_size] = 0;
    fread(blk, file_size, 1, fh);
    fclose(fh);

    EyString ret = ey_runtime_string_create_literal(ctx, blk);
    ey_runtime_manual_free(blk);
    return ret;
}

EyString ey_stdlib_readline(EyExecutionContext *ctx) {
    size_t size = 1024;
    char *buf = 0;
    getline(&buf, &size, stdin);
    return ey_runtime_string_create_literal(ctx, buf);
}



