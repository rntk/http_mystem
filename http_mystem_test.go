package http_mystem

import (
    "testing"
    "strings"
)

func TestProcessMystemOptions(t *testing.T) {
    type TStruct struct{
        opts []string
        ok_opts string
        ok_format string
    }
    var t_data = []TStruct{
        {[]string{"-c", "-s", "-d", "-e cp1251"}, "-s -n", "text"},
        {[]string{"-n", "--format json"}, "-n --format json", "json"},
        {[]string{"-n", "-w", "-l", "-i", "--eng-gr", "--format xml"}, "-n -w -l -i --eng-gr --format xml", "xml"},
    }
    for i, t_item := range t_data {
        opt, format, err := ProcessMystemOptions(t_item.opts)
        j_opt := strings.Join(opt, " ")
        if (j_opt != t_item.ok_opts) || (format != t_item.ok_format) || (err != nil) {
            t.Errorf("Failed on %v %v", i, t_item)
        }
    }
}
