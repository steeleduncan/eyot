/*
  Eyot threadsafe pipe implementation
 */
#pragma once

#include "eyot-runtime-common.h"

typedef struct EyPipe EyPipe;
EyPipe *ey_pipe_create(int value_size);
void *ey_pipe_at(EyPipe *p, int i);
void ey_pipe_send(EyPipe *p, const void *value);
EyBoolean ey_pipe_receive(EyPipe *p, void *value);
void ey_pipe_close(EyPipe *p);
EyVector *ey_pipe_receive_multiple(EyPipe *p, int count);
