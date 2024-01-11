package tools

import (
	"strconv"
	"strings"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/gin-gonic/gin"
)

type ExportDataEntry struct {
	Name  string
	XName string
	YName string
	Data  []dao.Stat
}

type ExportStatsContainer struct {
	data []*ExportDataEntry
}

const (
	CSVSep = ","
	CSVLb  = "\n\r"
)

func csvCell(val string) string {
	val = strings.ReplaceAll(val, "\"", "\"\"")
	return "\"" + val + "\""
}

func (e ExportStatsContainer) AddDataEntry(entry *ExportDataEntry) ExportStatsContainer {
	e.data = append(e.data, entry)
	return e
}

func (e ExportStatsContainer) ExportCsv() string {
	result := ""
	for _, data := range e.data {
		result += csvCell("Name") + CSVSep + csvCell(data.XName) + CSVSep + csvCell(data.YName) + CSVLb
		for _, stat := range data.Data {
			result += csvCell(data.Name) + CSVSep + csvCell(stat.X) + CSVSep + csvCell(strconv.Itoa(stat.Y)) + CSVLb
		}
	}
	return result
}

func (e ExportStatsContainer) ExportJson() []gin.H {
	var result []gin.H
	for _, data := range e.data {
		var stats []gin.H

		xName := strings.ToLower(data.XName)
		yName := strings.ToLower(data.YName)

		for _, stat := range data.Data {
			stats = append(stats, gin.H{
				xName: stat.X,
				yName: stat.Y,
			})
		}

		result = append(result, gin.H{"name": data.Name, "data": stats})
	}
	return result
}
