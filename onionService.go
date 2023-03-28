package onionService

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"github.com/coolshithere/control"
	"github.com/coolshithere/process"
	
)

// onionSerivce is the wrapper around the onionSerivce process and control port connection. It
// should be created with Start and developers should always call Close when
// done.
type onionSerivce struct {
	// Process is the onionSerivce instance that is running.
	Process process.Process

	// Control is the onionSerivce controller connection.
	Control *control.Conn

	// ProcessCancelFunc is the context cancellation func for the onionSerivce process.
	// It is used by Close and should not be called directly. This can be nil.
	ProcessCancelFunc context.CancelFunc

	// ControlPort is the port that Control is connected on. It is 0 if the
	// connection is an embedded control connection.
	ControlPort int

	// DataDir is the path to the data direconionSerivcey that onionSerivce is using.
	DataDir string

	// DeleteDataDirOnClose is true if, when Close is invoked, the entire
	// direconionSerivcey will be deleted.
	DeleteDataDirOnClose bool

	// DebugWriter is the writer used for debug logs, or nil if debug logs
	// should not be emitted.
	DebugWriter io.Writer

	// StopProcessOnClose, if true, will attempt to halt the process on close.
	StopProcessOnClose bool

	// GeoIPCreatedFile is the path, relative to DataDir, that was created from
	// StartConf.GeoIPFileReader. It is empty if no file was created.
	GeoIPCreatedFile string

	// GeoIPv6CreatedFile is the path, relative to DataDir, that was created
	// from StartConf.GeoIPFileReader. It is empty if no file was created.
	GeoIPv6CreatedFile string
}

// StartConf is the configuration used for Start when starting a onionSerivce instance. A
// default instance with no fields set is the default used for Start.
type StartConf struct {
	// ExePath is the path to the onionSerivce executable. If it is not present, "onionSerivce" is
	// used either locally or on the PATH. This is ignored if ProcessCreaonionSerivce is
	// set.
	ExePath string

	// ProcessCreaonionSerivce is the override to use a specific process creaonionSerivce. If set,
	// ExePath is ignored.
	ProcessCreaonionSerivce process.Creator

	// UseEmbeddedControlConn can be set to true to use
	// process.Process.EmbeddedControlConn() instead of creating a connection
	// via ControlPort. Note, this only works when ProcessCreaonionSerivce is an
	// embedded onionSerivce creaonionSerivce with version >= 0.3.5.x.
	UseEmbeddedControlConn bool

	// ControlPort is the port to use for the onionSerivce controller. If it is 0, onionSerivce
	// picks a port for use. This is ignored if UseEmbeddedControlConn is true.
	ControlPort int

	// DataDir is the direconionSerivcey used by onionSerivce. If it is empty, a temporary
	// direconionSerivcey is created in TempDataDirBase.
	DataDir string

	// TempDataDirBase is the parent direconionSerivcey that a temporary data direconionSerivcey
	// will be created under for use by onionSerivce. This is ignored if DataDir is not
	// empty. If empty it is assumed to be the current working direconionSerivcey.
	TempDataDirBase string

	// RetainTempDataDir, if true, will not set the created temporary data
	// direconionSerivcey to be deleted on close. This is ignored if DataDir is not
	// empty.
	RetainTempDataDir bool

	// DisableCookieAuth, if true, will not use the default SAFECOOKIE
	// authentication mechanism for the onionSerivce controller.
	DisableCookieAuth bool

	// DisableEagerAuth, if true, will not authenticate on Start.
	DisableEagerAuth bool

	// EnableNetwork, if true, will connect to the wider onionSerivce network on start.
	EnableNetwork bool

	// ExtraArgs is the set of extra args passed to the onionSerivce instance when
	// started.
	ExtraArgs []string

	// onionSerivcercFile is the onionSerivcerc file to set on start. If empty, a blank onionSerivcerc is
	// created in the data direconionSerivcey and is used instead.
	onionSerivcercFile string

	// DebugWriter is the writer to use for debug logs, or nil for no debug
	// logs.
	DebugWriter io.Writer

	// NoHush if true does not set --hush. By default --hush is set.
	NoHush bool

	// NoAutoSocksPort if true does not set "--SocksPort auto" as is done by
	// default. This means the caller could set their own or just let it
	// default to 9050.
	NoAutoSocksPort bool

	// GeoIPReader, if present, is called before start to copy geo IP files to
	// the data direconionSerivcey. Errors are propagated. If the ReadCloser is present,
	// it is copied to the data dir, overwriting as necessary, and then closed
	// and the appropriate command line argument is added to reference it. If
	// both the ReadCloser and error are nil, no copy or command line argument
	// is used for that version. This is called twice, once with false and once
	// with true for ipv6.
	//
	// This can be set to onionSerivceutil/geoipembed.GeoIPReader to use an embedded
	// source.
	GeoIPFileReader func(ipv6 bool) (io.ReadCloser, error)
}

// Start a onionSerivce instance and connect to it. If ctx is nil, context.Background()
// is used. If conf is nil, a default instance is used.
func Start(ctx context.Context, conf *StartConf, dir string) (*onionSerivce, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if conf == nil {
		conf = &StartConf{}
	}
	onionSerivce := &onionSerivce{DataDir: conf.DataDir, DebugWriter: conf.DebugWriter, StopProcessOnClose: true}
	// Create the data dir and make it absolute
	if onionSerivce.DataDir == "" {
		tempBase := dir
		if tempBase == "" {
			tempBase = "."
		}
		var err error
		if tempBase, err = filepath.Abs(tempBase); err != nil {
			return nil, err
		}
		if onionSerivce.DataDir, err = ioutil.TempDir(tempBase, "data-dir-"); err != nil {
			return nil, fmt.Errorf("Unable to create temp data dir: %v", err)
		}
		onionSerivce.Debugf("Created temp data direconionSerivcey at: %v", onionSerivce.DataDir)
		onionSerivce.DeleteDataDirOnClose = !conf.RetainTempDataDir
	} else if err := os.MkdirAll(onionSerivce.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("Unable to create data dir: %v", err)
	}

	// !!!! From this point on, we must close onionSerivce if we error !!!!

	// Copy geoip stuff if necessary
	err := onionSerivce.copyGeoIPFiles(conf)
	// Start onionSerivce
	if err == nil {
		err = onionSerivce.startProcess(ctx, conf)
	}
	// Connect the controller
	if err == nil {
		err = onionSerivce.connectController(ctx, conf)
	}
	// Attempt eager auth w/ no password
	if err == nil && !conf.DisableEagerAuth {
		err = onionSerivce.Control.Authenticate("")
	}
	// If there was an error, we have to try to close here but it may leave the process open
	if err != nil {
		if closeErr := onionSerivce.Close(); closeErr != nil {
			err = fmt.Errorf("Error on start: %v (also got error trying to close: %v)", err, closeErr)
		}
	}
	return onionSerivce, err
}

func (t *onionSerivce) copyGeoIPFiles(conf *StartConf) error {
	if conf.GeoIPFileReader == nil {
		return nil
	}
	if r, err := conf.GeoIPFileReader(false); err != nil {
		return fmt.Errorf("Unable to read geoip file: %v", err)
	} else if r != nil {
		t.GeoIPCreatedFile = "geoip"
		if err := createFile(filepath.Join(t.DataDir, "geoip"), r); err != nil {
			return fmt.Errorf("Unable to create geoip file: %v", err)
		}
	}
	if r, err := conf.GeoIPFileReader(true); err != nil {
		return fmt.Errorf("Unable to read geoip6 file: %v", err)
	} else if r != nil {
		t.GeoIPv6CreatedFile = "geoip6"
		if err := createFile(filepath.Join(t.DataDir, "geoip6"), r); err != nil {
			return fmt.Errorf("Unable to create geoip6 file: %v", err)
		}
	}
	return nil
}

func createFile(to string, from io.ReadCloser) error {
	f, err := os.Create(to)
	if err == nil {
		_, err = io.Copy(f, from)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if closeErr := from.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (t *onionSerivce) startProcess(ctx context.Context, conf *StartConf) error {
	// Get the creaonionSerivce
	creaonionSerivce := conf.ProcessCreaonionSerivce
	if creaonionSerivce == nil {
		onionSerivcePath := conf.ExePath
		if onionSerivcePath == "" {
			onionSerivcePath = "onionSerivce"
		}
		creaonionSerivce =process.NewCreator(onionSerivcePath)
	}
	// Build the args
	args := []string{"--DataDireconionSerivcey", t.DataDir}
	if !conf.DisableCookieAuth {
		args = append(args, "--CookieAuthentication", "1")
	}
	if !conf.EnableNetwork {
		args = append(args, "--DisableNetwork", "1")
	}
	if !conf.NoHush {
		args = append(args, "--hush")
	}
	if !conf.NoAutoSocksPort {
		args = append(args, "--SocksPort", "auto")
	}
	if t.GeoIPCreatedFile != "" {
		args = append(args, "--GeoIPFile", filepath.Join(t.DataDir, t.GeoIPCreatedFile))
	}
	if t.GeoIPv6CreatedFile != "" {
		args = append(args, "--GeoIPv6File", filepath.Join(t.DataDir, t.GeoIPv6CreatedFile))
	}
	// If there is no onionSerivcerc file, create a blank temp one
	onionSerivcercFileName := conf.onionSerivcercFile
	if onionSerivcercFileName == "" {
		onionSerivcercFile, err := ioutil.TempFile(t.DataDir, "onionSerivcerc-")
		if err != nil {
			return err
		}
		onionSerivcercFileName = onionSerivcercFile.Name()
		if err = onionSerivcercFile.Close(); err != nil {
			return err
		}
	}
	args = append(args, "-f", onionSerivcercFileName)
	// Create file for onionSerivce to write the control port to if it's not told to us and we're not embedded
	var controlPortFileName string
	var err error
	if !conf.UseEmbeddedControlConn {
		if conf.ControlPort == 0 {
			controlPortFile, err := ioutil.TempFile(t.DataDir, "control-port-")
			if err != nil {
				return err
			}
			controlPortFileName = controlPortFile.Name()
			if err = controlPortFile.Close(); err != nil {
				return err
			}
			args = append(args, "--ControlPort", "auto", "--ControlPortWriteToFile", controlPortFile.Name())
		} else {
			args = append(args, "--ControlPort", strconv.Itoa(conf.ControlPort))
		}
	}
	// Create process creaonionSerivce with args
	var processCtx context.Context
	processCtx, t.ProcessCancelFunc = context.WithCancel(ctx)
	args = append(args, conf.ExtraArgs...)
	p, err := creaonionSerivce.New(processCtx, args...)
	if err != nil {
		return err
	}
	// Use the embedded conn if requested
	if conf.UseEmbeddedControlConn {
		t.Debugf("Using embedded control connection")
		conn, err := p.EmbeddedControlConn()
		if err != nil {
			return fmt.Errorf("Unable to get embedded control conn: %v", err)
		}
		t.Control = control.NewConn(textproto.NewConn(conn))
		t.Control.DebugWriter = t.DebugWriter
	}
	// Start process with the args
	t.Debugf("Starting onionSerivce with args %v", args)
	if err = p.Start(); err != nil {
		return err
	}
	t.Process = p
	// If not embedded, try a few times to read the control port file if we need to
	if !conf.UseEmbeddedControlConn {
		t.ControlPort = conf.ControlPort
		if t.ControlPort == 0 {
		ControlPortCheck:
			for i := 0; i < 10; i++ {
				select {
				case <-ctx.Done():
					err = ctx.Err()
					break ControlPortCheck
				default:
					// Try to read the controlport file, or wait a bit
					var byts []byte
					if byts, err = ioutil.ReadFile(controlPortFileName); err != nil {
						break ControlPortCheck
					} else if t.ControlPort, err = process.ControlPortFromFileContents(string(byts)); err == nil {
						break ControlPortCheck
					}
					time.Sleep(200 * time.Millisecond)
				}
			}
			if err != nil {
				return fmt.Errorf("Unable to read control port file: %v", err)
			}
		}
	}
	return nil
}

func (t *onionSerivce) connectController(ctx context.Context, conf *StartConf) error {
	// This doesn't apply if already connected (e.g. using embedded conn)
	if t.Control != nil {
		return nil
	}
	t.Debugf("Connecting to control port %v", t.ControlPort)
	textConn, err := textproto.Dial("tcp", "127.0.0.1:"+strconv.Itoa(t.ControlPort))
	if err != nil {
		return err
	}
	t.Control = control.NewConn(textConn)
	t.Control.DebugWriter = t.DebugWriter
	return nil
}

// EnableNetwork sets DisableNetwork to 0 and optionally waits for bootstrap to
// complete. The context can be nil. If DisableNetwork isnt 1, this does
// nothing.
func (t *onionSerivce) EnableNetwork(ctx context.Context, wait bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// Only enable if DisableNetwork is 1
	if vals, err := t.Control.GetConf("DisableNetwork"); err != nil {
		return err
	} else if len(vals) == 0 || vals[0].Key != "DisableNetwork" || vals[0].Val != "1" {
		return nil
	}
	// Enable the network
	if err := t.Control.SetConf(control.KeyVals("DisableNetwork", "0")...); err != nil {
		return nil
	}
	// If not waiting, leave
	if !wait {
		return nil
	}
	// Wait for progress to hit 100
	_, err := t.Control.EventWait(ctx, []control.EventCode{control.EventCodeStatusClient},
		func(evt control.Event) (bool, error) {
			if status, _ := evt.(*control.StatusEvent); status != nil && status.Action == "BOOTSTRAP" {
				if status.Severity == "NOTICE" && status.Arguments["PROGRESS"] == "100" {
					return true, nil
				} else if status.Severity == "ERR" {
					return false, fmt.Errorf("Failing bootstrapping, onionSerivce warning: %v", status.Arguments["WARNING"])
				}
			}
			return false, nil
		})
	return err
}

// Close sends a halt to the onionSerivce process if it can, closes the controller
// connection, and stops the process.
func (t *onionSerivce) Close() error {
	t.Debugf("Closing onionSerivce")
	errs := []error{}
	// If controller is authenticated, send the quit signal to the process. Otherwise, just close the controller.
	sentHalt := false
	if t.Control != nil {
		if t.Control.Authenticated && t.StopProcessOnClose {
			if err := t.Control.Signal("HALT"); err != nil {
				errs = append(errs, fmt.Errorf("Unable to signal halt: %v", err))
			} else {
				sentHalt = true
			}
		}
		// Now close the controller
		if err := t.Control.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Unable to close contrlller: %v", err))
		} else {
			t.Control = nil
		}
	}
	if t.Process != nil {
		// If we didn't halt, we have to force kill w/ the cancel func
		if !sentHalt && t.StopProcessOnClose {
			t.ProcessCancelFunc()
		}
		// Wait for a bit to make sure it stopped
		errCh := make(chan error, 1)
		var waitErr error
		go func() { errCh <- t.Process.Wait() }()
		select {
		case waitErr = <-errCh:
			if waitErr != nil {
				errs = append(errs, fmt.Errorf("Process wait failed: %v", waitErr))
			}
		case <-time.After(300 * time.Millisecond):
			errs = append(errs, fmt.Errorf("Process did not exit after 300 ms"))
		}
		if waitErr == nil {
			t.Process = nil
		}
	}
	// Get rid of the entire data dir
	if t.DeleteDataDirOnClose {
		if err := os.RemoveAll(t.DataDir); err != nil {
			errs = append(errs, fmt.Errorf("Failed to remove data dir %v: %v", t.DataDir, err))
		}
	}
	// Combine the errors if present
	if len(errs) == 0 {
		return nil
	} else if len(errs) == 1 {
		t.Debugf("Error while closing onionSerivce: %v", errs[0])
		return errs[0]
	}
	t.Debugf("Errors while closing onionSerivce: %v", errs)
	return fmt.Errorf("Got %v errors while closing - %v", len(errs), errs)
}
