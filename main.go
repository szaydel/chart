package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
)

func main() {
	var fh *os.File
	var err error

	o := mustResolveOptions(os.Args[1:])

	_, o, b, err := buildChart(os.Stdin, o)
	if err == nil && b.Len() == 0 {
		os.Exit(0)
	}
	if err != nil {
		log.Fatal(err)
	}

	if strings.Compare(o.fileName, "") == 0 {
		fh, err = ioutil.TempFile("", "chartData")
		if err != nil {
			log.WithField("err", err).Fatalf("Could not create temporary file to store the chart.")
		}

		if _, err = fh.WriteString(baseTemplateHeaderString); err != nil {
			log.WithField("err", err).Fatalf("Could not write header to temporary file.")
		}
	} else {
		fh, err = os.Create(o.fileName)
		if err != nil {
			log.WithField("err", err).Fatalf(fmt.Sprintf("Could not create file %s to store the chart.", o.fileName))
		}
		if _, err = fh.WriteString(baseTemplateHeaderString); err != nil {
			log.WithField("err", err).Fatalf(fmt.Sprintf("Could not write header to file %s.", o.fileName))
		}
	}

	t := baseTemplate
	if o.chartType == pie {
		t = basePieTemplate
	}
	if err = t.Execute(fh, b.String()); err != nil {
		log.WithField("err", err).Fatalf("Could not write chart to file.")
	}

	if _, err = fh.WriteString(baseTemplateFooterString); err != nil {
		log.WithField("err", err).Fatalf("Could not write footer to file.")
	}

	if err = fh.Close(); err != nil {
		log.WithField("err", err).Fatalf("Could not close file after saving chart to it.")
	}

	if strings.Compare(o.fileName, "") == 0 {
		newName := fh.Name() + ".html"
		if err = os.Rename(fh.Name(), newName); err != nil {
			log.WithField("err", err).Fatalf("Could not add html extension to the temporary file.")
		}
		open.Run("file://" + newName)
	} else {
		open.Run("file://" + o.fileName)
	}
}

func buildChart(r io.Reader, o options) ([]string, options, bytes.Buffer, error) {
	d, o, lf, ls := preprocess(r, o)
	var b bytes.Buffer

	if o.debug {
		showDebug(ls, d, o, lf)
		return ls, o, b, nil
	}

	var err error
	var templ *template.Template
	var templData interface{}

	switch o.chartType {
	case pie:
		if len(d.fss) == 0 || (len(d.fss[0]) == 1 && len(d.sss) == 0 && len(d.tss) == 0) {
			return ls, o, b, fmt.Errorf("couldn't find values to plot")
		}
	case bar:
		if len(d.fss) == 0 || (len(d.fss[0]) == 1 && len(d.sss) == 0 && len(d.tss) == 0) {
			return ls, o, b, fmt.Errorf("couldn't find values to plot")
		}
	case line:
		if d.fss == nil || (d.sss == nil && d.tss == nil && len(d.fss[0]) < 2) {
			return ls, o, b, fmt.Errorf("couldn't find values to plot")
		}
	case scatter:
		if len(d.fss) == 0 {
			return ls, o, b, fmt.Errorf("couldn't find values to plot")
		}
	}
	if err != nil {
		return ls, o, b, fmt.Errorf("could not construct chart because [%v]", err)
	}

	templData, templ, err = cjsChart{inData{
		ChartType: o.chartType.string(),
		FSS:       d.fss,
		SSS:       d.sss,
		TSS:       d.tss,
		MinFSS:    d.minFSS,
		MaxFSS:    d.maxFSS,
		Title:     o.title,
		ScaleType: o.scaleType.string(),
		XLabel:    o.xLabel,
		YLabel:    o.yLabel,
		ZeroBased: o.zeroBased,
	}}.chart()

	if err := templ.Execute(&b, templData); err != nil {
		return ls, o, b, fmt.Errorf("could not prepare ChartJS js code for chart: [%v]", err)
	}

	return ls, o, b, nil
}
