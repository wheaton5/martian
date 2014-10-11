mario
=====

How to Clone Me
---------------
This repo includes vendored third-party code, so it must be git cloned in a specific way:

```
> cd $GOPATH
> git clone git@github.com:10XDev/mario.git src --recursive
```
Make sure to clone into a folder named `src` directly under your `$GOPATH`, not into a folder named `mario`.


Mario Executables
-----------------
To view commandline usage and options for any executable, give the `--help` option.

- `mrc` Commandline MRO compiler. Checks syntax and semantics of MRO files.
- `mre` Visual IDE for editing and compiling MRO files, and visualizing pipeline structure.
- `mrf` Commandline canonical code formatter for MRO files.
- `mrp` Pipeline runner.
- `mrs` Single-stage runner.
- - `marsoc` MARSOC (aka Mario Sequencing Operations Command)
