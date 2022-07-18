# xk6-timers

This extension adds a PoC implementaion of setTimeout and friends based on code by @na--

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
