# xk6-timers


> [!WARNING]  
> Starting k6 version v0.51 the code of the `k6/x/timers` extension is part of the [main k6 repository](https://github.com/grafana/k6). It can be used without any imports as in other js runtimes, or by importing `k6/timers`. Please contribute and [open issues there](https://github.com/grafana/k6/issues). This repository is no longer maintained.

This extension adds a PoC implementation of setTimeout and friends based on code by @na--

It is implemented using the [xk6](https://k6.io/blog/extending-k6-with-xk6/) system.

## Getting started  

1. Install `xk6`:
  ```shell
  $ go install go.k6.io/xk6/cmd/xk6@latest
  ```

2. Build the binary:
  ```shell
  $ xk6 build --with github.com/grafana/xk6-timers
  ```

## Examples

See ./mdn_example.js
