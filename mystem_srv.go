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

type Config struct {
    Host string
    Port int
    Reg_filter string
    Mystem_path string
    Mystem_workers int
    Channel_buffer int
    Max_word_length int
    Max_words int
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
        }
    }
    return cfg, err
}

func workerMystem(for_process chan Data, mystem_path string) int {
    //name := rand.Int()
    //log.Print("Mystem started ", name)
    mystem := exec.Command(mystem_path, "-n", "--format", "json")
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
        for {
            data = <- for_process
            n, err = mystem_writer.Write([]byte(fmt.Sprintf("%v\n", data.word)))
            if err != nil {
                log.Panic(fmt.Sprintf("Can't send word to mystem: %v", err))
            }
            buf = make([]byte, 1000)
            n, err = mystem_reader.Read(buf)
            if err != nil {
                log.Panic(fmt.Sprintf("Can't read answer from mystem: %v", err))
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

func checkInput(req *http.Request) (words []string, code int) {
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
                words = append(words, w)
            }
        }
    }
    return words, code
}

func processWords(resp http.ResponseWriter, req *http.Request) {
    var data Data
    var answer Answer
    var request_answer string = ""
    req.ParseForm()
    words, status_code := checkInput(req)
    var words_count int = len(words)
    resp.Header().Set("Content-Type", "application/json;charset=utf-8")
    if words_count > 0 {
        request_answer = "["
        var local_channel chan Answer = make(chan Answer, words_count)
        data.channel = local_channel
        var start int = 0
        for _, word := range words {
            if reg_filter.MatchString(word) {
                data.word = word
                for_process <- data
            } else {
                start++
                request_answer += fmt.Sprintf(`{"analysis":[], "text":"%v"},`, word)
            }
        }
        for i := start; i < words_count; i++ {
            answer = <- local_channel
            if (i + 1) < words_count {
                answer.data += ","
            }
            request_answer += answer.data
        }
        request_answer += "]"
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
        log.Print(fmt.Sprintf("Server start on: %v:%v", config.Host, config.Port))
        err = http.ListenAndServe(fmt.Sprintf("%v:%v", config.Host, config.Port), nil)
        if err != nil {
            makePanic(fmt.Sprintf("Can't start http server: %v", err))
        }
    }
}