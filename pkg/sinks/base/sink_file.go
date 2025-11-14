package base

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/model"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/sinks"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
)

type Configuration struct {
	FileName string `json:"file_name"`
}

// FileSink represents an outbound connector that writes streaming data
// to a file.
type FileSink struct {
	fileName string
	in       chan any
	done     chan struct{}
}

var _ streams.Sink = (*FileSink)(nil)

// NewFileSink returns a new FileSink connector.
func NewFileSink(fileName string) *FileSink {
	fileSink := &FileSink{
		fileName: fileName,
		in:       make(chan any),
		done:     make(chan struct{}),
	}

	// asynchronously process stream data
	go fileSink.process()

	return fileSink
}

// process reads data from the input channel and writes it to the file.
func (fs *FileSink) process() {
	defer close(fs.done)

	file, err := os.Create(fs.fileName)
	if err != nil {
		slog.Error("Failed to create file",
			slog.String("name", fs.fileName),
			slog.Any("error", err))

		// discard buffered input elements
		drainChan(fs.in)

		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("Failed to close file",
				slog.String("name", fs.fileName),
				slog.Any("error", err))
		}
	}()

	for element := range fs.in {
		var stringElement string
		switch v := element.(type) {
		case string:
			stringElement = v
		case []byte:
			stringElement = string(v)
		case fmt.Stringer:
			stringElement = v.String()
		default:
			slog.Warn("Discarded stream element",
				slog.String("type", fmt.Sprintf("%T", v)))
			continue
		}

		_, err := file.WriteString(stringElement)
		// Write the processed string element to the file. Use the specified
		// retry function to retry if an error occurs.
		// If failed to write, cancel the source context, drain the input
		// channel and terminate the stream processing.
		if err != nil {
			slog.Error("Failed to write to file",
				slog.String("name", fs.fileName),
				slog.Any("error", err))

			// discard buffered input elements
			drainChan(fs.in)
		}
	}
}

// In returns the input channel of the FileSink connector.
func (fs *FileSink) In() chan<- any {
	return fs.in
}

// AwaitCompletion blocks until the FileSink has completed processing and
// flushing all data to the file.
func (fs *FileSink) AwaitCompletion() {
	<-fs.done
}

func init() {
	sinks.RegistrySingleton.Register("file", &FileConverter{})
}

type FileConverter struct {
	sinks.Converter
}

func (c *FileConverter) Convert(input model.OutputSpec) (streams.Sink, error) {
	// marshal to json, unmarshal to config
	data, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, err
	}
	conf := &Configuration{}
	err = json.Unmarshal(data, conf)
	if err != nil {
		return nil, err
	}
	conf = &Configuration{
		FileName: conf.FileName,
	}
	src := NewFileSink(conf.FileName)
	return src, nil
}
