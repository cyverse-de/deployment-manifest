package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	dockerURI  = flag.String("docker-uri", "unix:///var/run/docker.sock", "The docker URI.")
	repoTags   = flag.String("repo-tags", "", "A CSV record telling with Docker repo tags to generate a manifest from.")
	outputPath = flag.String("output", "", "The file to write the JSON to.")
)

// ImageInfo contains the information about each image that needs to be included
// in the manifest.
type ImageInfo struct {
	RepoTag string `json:"repo-tag"`
	ImageID string `json:"image-id"`
	GitRef  string `json:"git-ref"`
}

// OutputMap is the top-level struct that the JSON output is marshalled from.
type OutputMap struct {
	Hostname string      `json:"hostname"`
	Date     string      `json:"date"`
	Images   []ImageInfo `json:"images"`
}

func parseRepoTags(input string) ([]string, error) {
	var repoTags []string
	reader := csv.NewReader(strings.NewReader(input))
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			repoTags = append(repoTags, f)
		}
	}
	return repoTags, nil
}

func main() {
	flag.Parse()

	if *repoTags == "" {
		log.Fatal("--repo-tags must be set.")
	}

	if *outputPath == "" {
		log.Fatal("--repo-tags must be set.")
	}

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	d, err := client.NewClient(*dockerURI, "v1.22", nil, defaultHeaders)
	if err != nil {
		log.Fatalf("Error creating docker client: %s", err)
	}

	parsedRepos, err := parseRepoTags(*repoTags)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// do a docker pull on all of the images that the user specified.
	var body io.ReadCloser
	for _, ref := range parsedRepos {
		body, err = d.ImagePull(ctx, ref, types.ImagePullOptions{})
		defer body.Close()
		if err != nil {
			log.Fatal(err)
		}

		if _, err = io.Copy(os.Stderr, body); err != nil {
			log.Fatal(err)
		}
	}

	// grab a list of all of the local images.
	listedImages, err := d.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		log.Fatal(err)
	}

	// Only include an image in the output if one of its RepoTags is included
	// in the list of images that the user specified on the command-line.
	var imageInfos []ImageInfo
	for _, li := range listedImages {
		for _, listedRepoTag := range li.RepoTags {
			for _, rt := range parsedRepos {
				if listedRepoTag == rt {
					var gitref string
					if _, ok := li.Labels["org.cyverse.git-ref"]; ok {
						gitref = li.Labels["org.cyverse.git-ref"]
					}
					imageInfos = append(imageInfos, ImageInfo{
						RepoTag: listedRepoTag,
						ImageID: li.ID,
						GitRef:  gitref,
					})
				}
			}
		}
	}

	var hostname string
	if hostname, err = os.Hostname(); err != nil {
		hostname = ""
	}

	date := time.Now().Format("2006-01-02T15:04:05-07:00")

	output := &OutputMap{
		Hostname: hostname,
		Date:     date,
		Images:   imageInfos,
	}

	imgJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %s", err)
	}

	of, err := os.Create(*outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer of.Close()

	if _, err = of.Write(imgJSON); err != nil {
		log.Fatal(err)
	}

}
