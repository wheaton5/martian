martian
=======

How to Clone Me
---------------
This repo includes vendored third-party code as submodules, so it must be git cloned recursively:

```
> git clone git@github.com:10XDev/martian.git --recursive
```

Martian Executables
-------------------
To view commandline usage and options for any executable, give the `--help` option.

- `mrc` Commandline MRO compiler. Checks syntax and semantics of MRO files.
- `mre` Visual IDE for editing and compiling MRO files, and visualizing pipeline structure.
- `mrf` Commandline canonical code formatter for MRO files.
- `mrp` Pipeline runner.
- `mrs` Single-stage runner.
- `marsoc` MARSOC (aka Martian Sequencing Operations Command)

