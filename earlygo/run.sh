#!/bin/sh
exec docker run -i -t -p 6060:6060  -h earlygo -w /home/gopher gc14:earlygo /bin/bash

