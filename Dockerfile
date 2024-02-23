# Build Stage
FROM golang:1.18 AS builder

RUN echo "validating current branch"