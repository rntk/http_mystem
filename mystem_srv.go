package main

import (
    "fmt"
    "net/http"
    "os"
    "os/exec"
    "encoding/json"
    "log"
    "time"
    "strings"
    //"math/rand"
    "regexp"
)

var HEADER_CONTENT_TYPES map[string]string = map[string]string{
    "json": "javascript/json;charset=utf-8",
    "text": "text/plain;charset=utf-8",
    "xml": "text/xml;charset=utf-8",
}

const DEFAULT_FORMAT string = "text"

type Config struct {
    Host string
    Port int
    Reg_filter string
    Mystem_path string
    Mystem_options []string
    Mystem_workers int
    Channel_buffer int
    Max_word_length int
    Max_words int
    format string
}

type Answer struct {
    data string
    err error
}

type Data struct {
    word string
    channel chan Answer
}

var for_process chan Data
var reg_filter *regexp.Regexp
var config Config

func makePanic(msg string) {
    log.Fatal(msg)
    panic(msg)
}

func processMystemOptions(in_opts []string) (out_opts []string, format string, err error) {
    var opts_arr []string
    var approved bool = false
    var exists bool = false
    for _, opt := range in_opts {
        opt = strings.TrimSpace(opt)
        if strings.Index(opt, " ") == -1 {
            opts_arr = make([]string, 1)
            opts_arr[0] = opt
            if opt == "-e" {
                approved = false
            } else {
                approved = true
            }
        } else {
            opts_arr = make([]string, 2)
            opts_arr = strings.Split(opt, " ")
            if opts_arr[0] == "--format" {
                _, exists = HEADER_CONTENT_TYPES[opts_arr[1]]
                if exists {
                    format = opts_arr[1]
                } else {
                    format = DEFAULT_FORMAT
                    log.Printf("Mystem option '%v' was replaced by default value. Because contain error")
                }
                approved = true
            } else if opts_arr[0] != "-e"{
                approved = true
            }
        }
        if approved {
            out_opts = append(out_opts, opts_arr...)
        } else {
            log.Printf("Mystem option '%v' was ignored or replaced by default value. Because wrong or unnecessary", opt)
        }
    }
    if format == "" {
        format = DEFAULT_FORMAT
    }
    return out_opts, format, err
}

func loadConfig() (cfg Config, err error) {
    var f_name string = "conf.json"
    if (len(os.Args) > 1) {
        f_name = os.Args[1]
    }
    file, err := os.Open(f_name)
    defer file.Close()
    if err == nil {
        f_stat, _ := file.Stat()
        raw_json := make([]byte, f_stat.Size())
        _, err = file.Read(raw_json)
        if err == nil {
            err = json.Unmarshal(raw_json, &cfg)
            if err == nil {
                cfg.Mystem_options, cfg.format, err = processMystemOptions(cfg.Mystem_options)
            }
        }
    }
    return cfg, err
}

func workerMystem(for_process chan Data, mystem_path string) int {
    //name := rand.Int()
    //log.Print("Mystem started ", name)
    mystem := exec.Command(mystem_path, config.Mystem_options...)
    mystem_writer, err_w := mystem.StdinPipe()
    mystem_reader, err_r := mystem.StdoutPipe()
    err := mystem.Start()
    if (err_w != nil) || (err_r != nil) {
        makePanic(fmt.Sprintf("Can't start: \n%v\n%v", err_w, err_r))
    }
    if err != nil {
        makePanic(fmt.Sprintf("Can't start: %v", err))
    } else {
        var data Data
        var buf []byte
        var n int
        var answer Answer
        if config.format == "xml" {
            n, err = mystem_writer.Write([]byte("test"))
            if err != nil {
                makePanic(fmt.Sprintf("Can't send word to mystem: %v", err))
            }
            buf = make([]byte, 1000)
            n, err = mystem_reader.Read(buf)
            if err != nil {
                makePanic(fmt.Sprintf("Can't send word to mystem: %v", err))
            }
        }
        for {
            data = <- for_process
            n, err = mystem_writer.Write([]byte(fmt.Sprintf("%v\n", data.word)))
            if err != nil {
                log.Panicf("Can't send word to mystem: %v", err)
            }
            buf = make([]byte, 1000)
            n, err = mystem_reader.Read(buf)
            if err != nil {
                log.Panicf("Can't read answer from mystem: %v", err)
            }
            answer.data = strings.TrimSpace(string(buf[:n]))
            answer.err = err
            //time.Sleep(time.Second * time.Duration(rand.Intn(5)))
            data.channel <- answer
            time.Sleep(time.Millisecond * 100)
        }
    }
    return 0
}

func checkInput(req *http.Request, words *[]string, ) (code int) {
    var words_count int = len(req.Form["words[]"])
    if words_count == 0 {
        code = http.StatusNotAcceptable
    } else if words_count > int(config.Max_words) {
        code = http.StatusRequestEntityTooLarge
    } else {
        var w string = ""
        for _, word := range req.Form["words[]"] {
            w = strings.TrimSpace(word)
            if len(w) > int(config.Max_word_length) {
                w = word[:int(config.Max_word_length)]
            }
            if w != "" {
                *words = append(*words, w)
            }
        }
    }
    return code
}

func processWords(resp http.ResponseWriter, req *http.Request) {
    var data Data
    var answer Answer
    var answers []string
    var request_answer string = ""
    req.ParseForm()
    var words []string
    var tmp_answer string = ""
    status_code := checkInput(req, &words)
    var words_count int = len(words)
    resp.Header().Set("Content-Type", HEADER_CONTENT_TYPES[config.format])
    if words_count > 0 {
        var local_channel chan Answer = make(chan Answer, words_count)
        data.channel = local_channel
        var start int = 0
        for _, word := range words {
            if reg_filter.MatchString(word) {
                data.word = word
                for_process <- data
            } else {
                start++
                switch config.format {
                    case "json": {
                        tmp_answer = fmt.Sprintf(`{"analysis":[], "text":"%v"},`, word)
                    }
                    case "xml": {
                        tmp_answer = "<w>" + word + "</w>"
                    }
                    case "text": {
                        tmp_answer = word
                    }
                }
                answers = append(answers, tmp_answer)
            }
        }
        for i := start; i < words_count; i++ {
            answer = <- local_channel
            fmt.Println(answer.data)
            answers = append(answers, answer.data)
        }
        switch config.format {
            case "json": {
                request_answer = "[" + strings.Join(answers, ",") + "]"
            }
            case "xml": {
                request_answer = `<?xml version="1.0" encoding="utf-8"?><html><body><se>` + strings.Join(answers, "\n") + "</se></body></html>"
            }
            case "text": {
                request_answer = strings.Join(answers, "\n")
            }
        }
        close(local_channel)
    } else {
        request_answer = fmt.Sprintf(`{"status": %v, "reason": "%v"}`, status_code, http.StatusText(status_code))
        resp.WriteHeader(status_code)
    }
    resp.Write([]byte(request_answer))
}

func main() {
    var err error = nil
    config, err = loadConfig()
    if err != nil {
        makePanic(fmt.Sprintf("Can't load config: %v", err))
    } else {
        reg_filter, err = regexp.Compile(config.Reg_filter)
        if err != nil {
            makePanic(fmt.Sprintf("Can`t compile regular expression: %v", err))
        }
        _, err = os.Open(config.Mystem_path)
        if  err != nil {
            makePanic(fmt.Sprintf("Can't find mystem: %v", err))
        }
        var i int
        if config.Channel_buffer > 0 {
            for_process = make(chan Data, int(config.Channel_buffer))
        } else {
            for_process = make(chan Data)
        }
        log.Print("Config load successful")
        for i = 0; i < int(config.Mystem_workers); i++ {
            go workerMystem(for_process, config.Mystem_path)
        }
        http.HandleFunc("/", processWords)
        log.Printf("Server start on: %v:%v", config.Host, config.Port)
        err = http.ListenAndServe(fmt.Sprintf("%v:%v", config.Host, config.Port), nil)
        if err != nil {
            makePanic(fmt.Sprintf("Can't start http server: %v", err))
        }
    }
}