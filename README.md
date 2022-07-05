# waPC CLI
 
WebAssembly Procedure Calls command-line interface. 
 
### Problem
 
WebAssembly is capable of passing and accepting simple numeric parameters between host and guests while non-trivial applications would like to leverage more complex data types like strings, structs, binary blobs, or other complex data types.
 
### Solution
 
waPC is a polyglot specification and toolkit for WebAssembly that enables a bidirectional function call mechanism to enable and simplify the passing of strings, structs, binary blobs, or other complex data types between host and guests systems as native language parameter types.

### Basic Overview

waPC cli is a simple and fast polyglot code generator for waPC.  waPC leverages a simple workflow, robust set of templates, and user customization to generate this scaffolding code and may be further customized to your use case.  It presently supports [AssemblyScript](https://www.assemblyscript.org/), [Rust](https://www.rust-lang.org/), and [TinyGo](https://tinygo.org/).  You may leverage this cli to create the scaffolding and libraries necessary to build your applications.  waPC internally leverages an Interactive Data Language (IDL) called WIDL, short for WebAssembly IDL, based on [GraphQL](https://graphql.org/learn/schema/), but with with many features omitted for simplicity.  Complex parameters are encoded using MessagePack which is simple, performs well, and easy to use/debug.

For further information about our design choices and architecture please see our FAQ.  While WIDL is used internally between the host and underlying guest, you are of course free to expose whatever IDL you would like externally via the API or other mechanisms.

waPC cli has a very simple workflow:

1. Generate a basic project and data model scaffold with your choice of language.
2. Customize templates.
3. Compile your auto-generated libraries.
4. Load your libraries in a waPC host (e.g. [wapc-rust](https://github.com/wapc/wapc-rust) or [wapc-go](https://github.com/wapc/wapc-go)) and leverage them in your project.
...
Profit

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

### QuickStart (YOLO)

#### Install the CLI

Windows

```
powershell -Command "iwr -useb https://raw.githubusercontent.com/wapc/cli/master/install/install.ps1 | iex"
```

MacOS

```
curl -fsSL https://raw.githubusercontent.com/wapc/cli/master/install/install.sh | /bin/bash
```

Linux

```
wget -q https://raw.githubusercontent.com/wapc/cli/master/install/install.sh -O - | /bin/bash
```

Homebrew

```
brew install wapc/tap/wapc
```

#### Create a waPC module

AssemblyScript

```shell
wapc new assemblyscript hello_world_as
cd hello_world_as
make
```

Rust

```shell
wapc new rust hello_world_rust
cd hello_world_rust
make
```

TinyGo

```shell
wapc new tinygo hello_world_tinygo
cd hello_world_tinygo
make codegen
go mod tidy
make build
```

The `build` directory will contain your `.wasm` file.

##### Note for users not using the standard NPM Registry

Set an environment variable called `NPM_REG` to set the registry host for the cli
```
NPM_REG="https://my.reg.com/npm" wapc new ...
```

### Basic Scaffolding

waPC cli has a very simple workflow:

1. Generate basic project and data model scaffold.
2. Customize templates.
3. Compile your libraries.
4. Load your libraries in a waPC host (e.g. [wapc-rust](https://github.com/wapc/wapc-rust) or [wapc-go](https://github.com/wapc/wapc-go)) and leverage them in your project.

Generate a new application

```shell
wapc new assemblyscript hello_world
```

Inspect your scaffold

```
hello_world
├── Makefile
├── assembly
│   └── tsconfig.json
├── codegen.yaml
├── package.json
└── schema.widl
```

### About the Template Files

The scaffolding created by the `wapc new` step above creates a template project that you can then use 'make' to build into your custom library.  You can customize this template project with your data specification, the files you would like the auto generator to build, and optionally your own custom templates.

1. `Makefile`: regardless of what language you use in the generator you can simply use `make` to build your project.
2. `codegen.yaml` is used to map generated files to their *module*, *visitorClass* and optional *config* settings.
3. `package.json` provides instructions to npm on which templates to download for the autogeneration and further build instructions for the generated code.
4. `schema.widl` is the data schema that you should customize for your application.
5. `assembly/tsconfig.json` is the typescript configuration to enable AssemblyScript in your editor.

### Building your module

Once you have customized your `codegen.yaml` and `schema.widl` you are ready to build your project:

```shell
make
```

npm will provide you with some feedback, download a bunch of packages, and then auto generate your library.  It should look a bit like this now:

```
hello_world
├── Makefile
├── assembly
│   ├── index.ts
│   ├── module.ts
│   └── tsconfig.json
├── build
│   └── hello_world.wasm
├── codegen.yaml
├── node_modules/*
├── package-lock.json
├── package.json
└── schema.widl
```

We of course have all of our node files under `node_modules` and our node configuration `package-lock.json`.

Our key autogenerated files:

1. `assembly/index.ts` our AssemblyScript entry point with stubbed out functions to implement. (only generated if it does not exist)
2. `assembly/module.ts` our AssemblyScript library. (generated each time with `make` or `make codegen`)
3. `build/hello_world.wasm` our WebAssembly library to be loaded into a wapc guest.

## Deployment

A standalone guide to deploying our `hello_world` application is coming soon. This will describe leveraging a waPC host library (e.g. [wapc-rust](https://github.com/wapc/wapc-rust) or [wapc-go](https://github.com/wapc/wapc-go)) to dynamically load `hello_world.wasm` in your project.

## Development

### Prerequisites

To use the waPC cli has been tested with Go 1.16. Because the CLI uses the [`embed`](https://golang.org/pkg/embed/) package introduced in Go 1.16, previous versions of Go will have compiler errors.

Verify you have Go 1.16+ installed

```shell
go version
```

If Go is not installed, [download and install Go 1.16+](https://golang.org/dl/) (brew installation is not recommended because of CGO linker warnings)

Clone the project from github

```shell
git clone https://github.com/wapc/cli.git
cd cli
go install ./cmd/...
```

**Compiling on Windows**

In order to build a project using v8go on Windows, Go requires a gcc compiler to be installed.

To set this up:
1. Install MSYS2 (https://www.msys2.org/)
2. Add the Mingw-w64 bin to your PATH environment variable (`C:\msys64\mingw64\bin` by default)
3. Open MSYS2 MSYS and execute `pacman -S mingw-w64-x86_64-toolchain`

V8 requires 64-bit on Windows, therefore it will not work on 32-bit systems. 

Confirm `wapc` runs (The Go installation should add `~/go/bin` in your `PATH`)

```shell
wapc --help
```

Output:

```
Usage: wapc <command>

Flags:
  -h, --help    Show context-sensitive help.

Commands:
  install <location> [<release>]
    Install a module.

  generate <config>
    Generate code from a configuration file.

  new <template> <dir> [<variables> ...]
    Creates a new project from a template.

  upgrade
    Upgrades to the latest base modules dependencies.

Run "wapc <command> --help" for more information on a command.
```

### WIDL and Code Generation

We are looking for help in enhancing the code generation modules to add more supported languages and plugins as well as general bug squashing. `wapc` will automatically download the [`widl-js`](https://github.com/wapc/widl-js) and [`widl-codegen-js`](https://github.com/wapc/widl-codegen-js) modules. To contribute to one of these modules, you should fork it and instruct `wapc` to install your fork.

```shell
wapc install github.com/<your username>/widl-codegen-js
```

To revert `widl-codegen-js` to the latest published version

```shell
wapc install @wapc/widl-codegen
```

You can also build independent modules and publish them to [NPM](npmjs.com).

```shell
wapc install <your optional npm org>/my-codegen-module
```

Your codegen modules should follow the same project structure as [widl-codegen-js](https://github.com/wapc/widl-codegen-js). See the documentation on the [base code generation modules](https://unpkg.com/@wapc/widl-codegen@0.0.3/docs/index.html). A tutorial for writing your own code generation module is coming...

## Reference projects

Let's look at a few example projects:

* [wasmCloud](https://www.wasmCloud.com) - A dynamic, elastically scalable WebAssembly host runtime for securely connecting actors and capability providers
* [Mandelbrot Example](https://github.com/wapc/mandelbrot-example) - an adaptation of AssemblyScript mandelbrot for waPC.
* [Rule Demo](https://github.com/wapc/rules-demo) - a simple rules engine for waPC
* [IBM Hyperledger](https://github.com/hyperledgendary/fabric-chaincode-wasm) - Smart Contracts, running in Wasm.  waPC Go Host is leveraged to execute the Wasm chaincode.

## Built With

* [waPC](https://github.com/wapc) - The WebAssembly Procedure Calls GitHub organization that contains host and guest libraries, this CLI, and all the supporting modules.
* [widl-js](https://github.com/wapc/widl-js) - Parser, AST, and Visitor pattern for the waPC Interface Definition Language (WIDL).
* [widl-codegen-js](https://github.com/wapc/widl-codegen-js) - Code generation library using waPC Interface Definition Language (WIDL).  Making your life, a *wittle* bit easier.
* [esbuild](https://esbuild.github.io/) - An extremely fast JavaScript bundler written in Go that is used to compile the code generation TypeScript modules into JavaScript that can run natively in V8.
* [v8go](https://github.com/rogchap/v8go) and [V8](https://v8.dev/) - Execute JavaScript from Go
* [kong](https://github.com/alecthomas/kong) - A very simple and easy to use command-line parser for Go
* [The Go 1.16 embed package](https://golang.org/pkg/embed/) - Finally embedding files is built into the Go toolchain!

## Contributing

Please read [CONTRIBUTING.md](https://github.com/wapc/cli/blob/main/CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/wapc/cli/tags).

## Contributors

* **Phil Kedy** - [pkedy](https://github.com/pkedy)

## License

This project is licensed under the [Apache License 2.0](https://choosealicense.com/licenses/apache-2.0/) - see the [LICENSE.txt](LICENSE.txt) file for details
