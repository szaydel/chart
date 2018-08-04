package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

type dataset struct {
	fss      [][]float64
	sss      [][]string
	tss      [][]time.Time
	minFSS   []float64
	maxFSS   []float64
	lf       string
	stdinLen int
}

func mustNewDataset(r io.Reader, o options) *dataset {
	d, err := newDataset(r, o)
	if err != nil {
		log.WithError(err).Fatal("Could not build dataset.")
	}
	return d
}

func newDataset(r io.Reader, o options) (*dataset, error) {
	d := &dataset{
		fss:    make([][]float64, 0, 500),
		sss:    make([][]string, 0, 500),
		tss:    make([][]time.Time, 0, 500),
		minFSS: make([]float64, 0, 500),
		maxFSS: make([]float64, 0, 500),
	}
	return d, d.read(r, o)
}

func (d *dataset) Len() int {
	if d.fss == nil {
		return len(d.tss)
	}
	return len(d.fss)
}

func (d *dataset) Less(i, j int) bool {
	if d.tss == nil {
		return d.fss[i][0] < d.fss[j][0]
	}
	return d.tss[i][0].Before(d.tss[j][0])
}

func (d *dataset) Swap(i, j int) {
	if d.fss != nil {
		d.fss[i], d.fss[j] = d.fss[j], d.fss[i]
	}
	if d.tss != nil {
		d.tss[i], d.tss[j] = d.tss[j], d.tss[i]
	}
	if d.sss != nil {
		d.sss[i], d.sss[j] = d.sss[j], d.sss[i]
	}
}

func (d *dataset) hasFloats() bool  { return len(d.fss) > 0 }
func (d *dataset) hasStrings() bool { return len(d.sss) > 0 }
func (d *dataset) hasTimes() bool   { return len(d.tss) > 0 }
func (d *dataset) timeFieldLen() int {
	if !d.hasTimes() {
		return 0
	}
	return len(d.tss[0])
}
func (d *dataset) floatFieldLen() int {
	if !d.hasFloats() {
		return 0
	}
	return len(d.fss[0])
}
func (d *dataset) canBeScatterLine() bool {
	return d.floatFieldLen()+d.timeFieldLen() >= 2
}

func (d *dataset) read(r io.Reader, o options) error {
	var (
		nilSSS, nilFSS, nilTSS = true, true, true
		scanner                = bufio.NewScanner(r)
		stdinLen               = 0
	)
	d.lf = o.lineFormat

	for scanner.Scan() {
		stdinLen++
		fs, ss, ts, err := d.parseLine(scanner.Text(), d.lf, o.separator, o.dateFormat)
		if err != nil {
			continue
		}
		if nilSSS && len(ss) > 0 {
			nilSSS = false
		}
		if nilFSS && len(fs) > 0 {
			nilFSS = false
		}
		if nilTSS && len(ts) > 0 {
			nilTSS = false
		}

		for i, f := range fs {
			if len(d.minFSS) == i {
				d.minFSS = append(d.minFSS, f)
			}
			if len(d.maxFSS) == i {
				d.maxFSS = append(d.maxFSS, f)
			}
			if f < d.minFSS[i] {
				d.minFSS[i] = f
			}
			if f > d.maxFSS[i] {
				d.maxFSS[i] = f
			}
		}

		d.fss = append(d.fss, fs)
		d.sss = append(d.sss, ss)
		d.tss = append(d.tss, ts)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	d.stdinLen = stdinLen
	if nilSSS {
		d.sss = nil
	}
	if nilFSS {
		d.fss = nil
		d.minFSS = nil
		d.maxFSS = nil
	}
	if nilTSS {
		d.tss = nil
	}
	if strings.Index(d.lf, "f") == -1 && len(d.sss) > 0 {
		d.fss, d.sss = preprocessFreq(d.sss)
	}
	if d.Len() == 0 {
		return fmt.Errorf("empty dataset; nothing to plot here")
	}

	return nil
}

func (d *dataset) parseLine(l string, lf string, sep rune, df string) ([]float64, []string, []time.Time, error) {
	l = string(regexp.MustCompile(string(sep)+"{2,}").ReplaceAll([]byte(l), []byte(string(sep))))
	sp := strings.Split(strings.TrimSpace(l), string(sep))

	fs := []float64{}
	ss := []string{}
	ds := []time.Time{}

	if len(sp) < len(lf) {
		return fs, ss, ds, fmt.Errorf("Input line has invalid format length; expected %v vs found %v", len(lf), len(sp))
	}

	for i, lfe := range lf {
		s := strings.TrimSpace(sp[i])
		switch lfe {
		case 's':
			ss = append(ss, s)
		case 'd':
			d, err := time.Parse(df, s)
			if err != nil {
				return fs, ss, ds, fmt.Errorf("Couldn't convert %v to date given: %v", s, err)
			}
			ds = append(ds, d)
		case 'f':
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return fs, ss, ds, fmt.Errorf("Couldn't convert %v to float given: %v", s, err)
			}
			fs = append(fs, f)
		}
	}

	return fs, ss, ds, nil
}
