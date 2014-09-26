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


Mario Executables Usage
-----------------------

```
Mario Compiler.

Usage:
    mrc <file.mro>... [--checksrcpath]
    mrc --all [--checksrcpath]
    mrc -h | --help | --version

Options:
    --all           Compile all files in $MROPATH.
    --checksrcpath  Check that stage source paths exist.
    -h --help       Show this message.
    --version       Show version.
```

```
Mario MRO Editor.

Usage:
    mre [--port=<num>]
    mre -h | --help | --version

Options:
    --port=<num>  Serve UI at http://localhost:<num>
                    Overrides $MROPORT_EDITOR environment variable.
                    Defaults to 3601 if not otherwise specified.
    --sge         Run jobs on Sun Grid Engine instead of locally.
    -h --help     Show this message.
    --version     Show version.
```

```
Mario Stage Runner.

Usage: 
    mrs <call.mro> [<stagestance_name>] [--sge]
    mrs -h | --help | --version

Options:
    --sge         Run jobs on Sun Grid Engine instead of locally.
    -h --help     Show this message.
    --version     Show version.
```

```
Mario Pipeline Runner.

Usage: 
    mrp <call.mro> <pipestance_name> [options]
    mrp -h | --help | --version

Options:
    --port=<num>     Serve UI at http://localhost:<num>
                       Overrides $MROPORT environment variable.
                       Defaults to 3600 if not otherwise specified.
    --noexit         Keep UI running after pipestance completes or fails.
    --noui           Disable UI.
    --novdr          Disable Volatile Data Removal.
    --profile        Enable stage performance profiling.
    --maxcores=<num> Set max cores the pipeline may request at one time.
    --maxmem=<num>   Set max GB the pipeline may request at one time.
    --sge            Run jobs on Sun Grid Engine instead of locally.
                     (--maxcores and --maxmem will be ignored)
    -h --help        Show this message.
    --version        Show version.
```
