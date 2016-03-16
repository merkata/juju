// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"fmt"

	"github.com/juju/cmd"
	"github.com/juju/loggo"
	"github.com/juju/utils"
	"launchpad.net/gnuflag"

	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/environs/filestorage"
	"github.com/juju/juju/environs/simplestreams"
	"github.com/juju/juju/environs/storage"
	envtools "github.com/juju/juju/environs/tools"
	"github.com/juju/juju/juju"
	"github.com/juju/juju/juju/osenv"
	coretools "github.com/juju/juju/tools"
)

func newToolsMetadataCommand() cmd.Command {
	return modelcmd.Wrap(&toolsMetadataCommand{})
}

// toolsMetadataCommand is used to generate simplestreams metadata for juju tools.
type toolsMetadataCommand struct {
	modelcmd.ModelCommandBase
	fetch       bool
	metadataDir string
	stream      string
	clean       bool
	public      bool
}

var toolsMetadataDoc = `
generate-tools creates simplestreams tools metadata.

This command works by scanning a directory for tools tarballs from which to generate
simplestreams tools metadata. The working directory is specified using the -d argument
(defaults to $JUJU_DATA or if not defined $XDG_DATA_HOME/juju or if that is not defined
~/.local/share/juju). The working directory is expected to contain a named subdirectory
containing tools tarballs, and is where the resulting metadata is written.

The stream for which metadata is generated is specified using the --stream parameter
(default is "released"). Metadata can be generated for any supported stream - released,
proposed, testing, devel.

Tools tarballs can are located in either a sub directory called "releases" (legacy),
or a directory named after the stream. By default, if no --stream argument is provided,
metadata for tools in the "released" stream is generated by scanning for tool tarballs
in the "releases" directory. By specifying a stream explcitly, tools tarballs are
expected to be located in a directory named after the stream.

Newly generated metadata will be merged with any exisitng metadata that is already there.
To first remove metadata for the specified stream before generating new metadata,
use the --clean option.

Examples:

  - generate metadata for "released" tools, looking in the "releases" directory:

   juju metadata generate-tools -d <workingdir>

  - generate metadata for "released" tools, looking in the "released" directory:

   juju metadata generate-tools -d <workingdir> --stream released

  - generate metadata for "proposed" tools, looking in the "proposed" directory:

   juju metadata generate-tools -d <workingdir> --stream proposed

  - generate metadata for "proposed" tools, first removing existing "proposed" metadata:

   juju metadata generate-tools -d <workingdir> --stream proposed --clean

`

func (c *toolsMetadataCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "generate-tools",
		Purpose: "generate simplestreams tools metadata",
		Doc:     toolsMetadataDoc,
	}
}

func (c *toolsMetadataCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.metadataDir, "d", "", "local directory in which to store metadata")
	// If no stream is specified, we'll generate metadata for the legacy tools location.
	f.StringVar(&c.stream, "stream", "", "simplestreams stream for which to generate the metadata")
	f.BoolVar(&c.clean, "clean", false, "remove any existing metadata for the specified stream before generating new metadata")
	f.BoolVar(&c.public, "public", false, "tools are for a public cloud, so generate mirrors information")
}

func (c *toolsMetadataCommand) Run(context *cmd.Context) error {
	loggo.RegisterWriter("toolsmetadata", cmd.NewCommandLogWriter("juju.environs.tools", context.Stdout, context.Stderr), loggo.INFO)
	defer loggo.RemoveWriter("toolsmetadata")
	if c.metadataDir == "" {
		c.metadataDir = osenv.JujuXDGDataHome()
	} else {
		c.metadataDir = context.AbsPath(c.metadataDir)
	}

	sourceStorage, err := filestorage.NewFileStorageReader(c.metadataDir)
	if err != nil {
		return err
	}

	// We now store the tools in a directory named after their stream, but the
	// legacy behaviour is to store all tools in a single "releases" directory.
	toolsDir := c.stream
	if c.stream == "" {
		fmt.Fprintf(context.Stdout, "No stream specified, defaulting to released tools in the releases directory.\n")
		c.stream = envtools.ReleasedStream
		toolsDir = envtools.LegacyReleaseDirectory
	}
	fmt.Fprintf(context.Stdout, "Finding tools in %s for stream %s.\n", c.metadataDir, c.stream)
	toolsList, err := envtools.ReadList(sourceStorage, toolsDir, -1, -1)
	if err == envtools.ErrNoTools {
		var source string
		source, err = envtools.ToolsURL(envtools.DefaultBaseURL)
		if err != nil {
			return err
		}
		toolsList, err = envtools.FindToolsForCloud(toolsDataSources(source), simplestreams.CloudSpec{}, c.stream, -1, -1, coretools.Filter{})
	}
	if err != nil {
		return err
	}

	targetStorage, err := filestorage.NewFileStorageWriter(c.metadataDir)
	if err != nil {
		return err
	}
	writeMirrors := envtools.DoNotWriteMirrors
	if c.public {
		writeMirrors = envtools.WriteMirrors
	}
	return mergeAndWriteMetadata(targetStorage, toolsDir, c.stream, c.clean, toolsList, writeMirrors)
}

func toolsDataSources(urls ...string) []simplestreams.DataSource {
	dataSources := make([]simplestreams.DataSource, len(urls))
	for i, url := range urls {
		dataSources[i] = simplestreams.NewURLSignedDataSource(
			"local source",
			url,
			juju.JujuPublicKey,
			utils.VerifySSLHostnames,
			simplestreams.CUSTOM_CLOUD_DATA,
			false)
	}
	return dataSources
}

// This is essentially the same as tools.MergeAndWriteMetadata, but also
// resolves metadata for existing tools by fetching them and computing
// size/sha256 locally.
func mergeAndWriteMetadata(
	stor storage.Storage, toolsDir, stream string, clean bool, toolsList coretools.List, writeMirrors envtools.ShouldWriteMirrors,
) error {
	existing, err := envtools.ReadAllMetadata(stor)
	if err != nil {
		return err
	}
	if clean {
		delete(existing, stream)
	}
	metadata := envtools.MetadataFromTools(toolsList, toolsDir)
	var mergedMetadata []*envtools.ToolsMetadata
	if mergedMetadata, err = envtools.MergeMetadata(metadata, existing[stream]); err != nil {
		return err
	}
	if err = envtools.ResolveMetadata(stor, toolsDir, mergedMetadata); err != nil {
		return err
	}
	existing[stream] = mergedMetadata
	return envtools.WriteMetadata(stor, existing, []string{stream}, writeMirrors)
}
