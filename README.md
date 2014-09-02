margo
=====

```
Mario Compiler.

Usage:
    mrc <file.mro>... | --all
    mrc -h | --help | --version

Options:
    --all         Compile all files in $MROPATH.
    -h --help     Show this message.
    --version     Show version.`
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
    --port=<num>  Serve UI at http://localhost:<num>
                    Overrides $MROPORT environment variable.
                    Defaults to 3600 if not otherwise specified.
    --cores=<num> Maximum number of cores to use in local mode.
    --noui        Disable UI.
    --novdr       Disable Volatile Data Removal.
    --sge         Run jobs on Sun Grid Engine instead of locally.
    -h --help     Show this message.
    --version     Show version.
```
