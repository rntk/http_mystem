package http_mystem

import (
    "fmt"
    "net/http"
    "os"
    "os/exec"
    "io"
    "encoding/json"
    "log"
    //"time"
    "strings"
    //"math/rand"
    "regexp"
)

var HEADER_CONTENT_TYPES map[string]string = map[string]string{
    "json": "javascript/json;charset=utf-8",
    "text": "text/plain;charset=utf-8",
    "xml": "text/xml;charset=utf-8",
}

var IGNORE_OPTIONS map[string]bool = map[string]bool{
    "-e": true,
    "-c": true,
    "-d": true,
}

const DEFAULT_FORMAT string = "text"

type Config struct {
    Host string
    Port int
    Reg_filter string
    Mystem_path string
    Mystem_options []string
    Mystem_workers int
    Mystem_answer_size int
    Channel_buffer int
    Max_word_length int
    Max_words int
    format string
    is_d bool
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

func ProcessMystemOptions(in_opts []string) (out_opts []string, format string, err error) {
    var opts_arr []string
    var approved bool = false
    var exists bool = false
    var n_exists = false
    for _, opt := range in_opts {
        approved = false
        opt = strings.TrimSpace(opt)
        opt = strings.ToLower(opt)
        if strings.Index(opt, " ") == -1 {
            opts_arr = make([]string, 1)
            opts_arr[0] = opt
            if !bool(IGNORE_OPTIONS[opt]) {
                approved = true
            }
            if (!n_exists) && (opt == "-n") {
                n_exists = true
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
                    opts_arr[1] = format
                    log.Printf("Mystem option '%v' was replaced by default value. Because contain error", opt)
                }
                approved = true
            } else if !bool(IGNORE_OPTIONS[opts_arr[0]]) {
                approved = true
            }
        }
        if approved {
            out_opts = append(out_opts, opts_arr...)
        } else {
            log.Printf("Mystem option '%v' was ignored or replaced by default value. Because wrong or unnecessary", opt)
        }
    }
    if !n_exists {
        out_opts = append(out_opts, "-n")
        log.Print("Added mystem option '-n'. It need for normal work.\n")
    }
    if format == "" {
        format = DEFAULT_FORMAT
    }
    return out_opts, format, err
}

func loadConfig() (cfg Config, err error) {
    var f_name string = "http_mystem.json"
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
                cfg.Mystem_options, cfg.format, err = ProcessMystemOptions(cfg.Mystem_options)
                //cfg.Mystem_answer_size = cfg.Max_word_length * 500
            }
        }
    }
    return cfg, err
}

func writeStringToPipe(for_write string, pipe io.WriteCloser) (n int, err error) {
    defer func() {
        result := recover()
        if result != nil {
            log.Fatalf("Recover say: %v", result)
        }
    }()    
    n, err = pipe.Write([]byte(for_write + "\n"))
    if err != nil {
        log.Panicf("Can't send word to mystem: %v", err)
    }
    return n, err
}

func readStringFromPipe(for_read *string, pipe io.ReadCloser) (n int, err error) {
    defer func() {
        result := recover()
        if result != nil {
            log.Fatalf("Recover say: %v", result)
        }
    }()
    buf := make([]byte, config.Mystem_answer_size)
    n, err = pipe.Read(buf)
    if err == nil {
        *for_read = strings.TrimSpace(string(buf[:n]))
    } else {
        log.Panicf("Can't send word to mystem: %v", err)
    } 
    return n, err
}

func readXMLTrash(number int, pipe io.ReadCloser) {
    var data string = ""
    var err error
    for i := 0; i < number; i++ {
        for data == "" {
            _, err = readStringFromPipe(&data, pipe)
            if err != nil {
                makePanic(fmt.Sprintf("Can't start: %v", err))
            }
        }
        data = ""
    }
}

func workerMystem(for_process chan Data, mystem_path string) {
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
        var answer Answer
        if config.format == "xml" {
            readXMLTrash(2, mystem_reader)
            _, err = writeStringToPipe("тест", mystem_writer)
            if err != nil {
                panic("")
            }
            readXMLTrash(2, mystem_reader)
        } else {
            _, err = writeStringToPipe("тест", mystem_writer)
            if err != nil {
                panic("")
            }
            _, err = readStringFromPipe(&answer.data, mystem_reader)
            if err != nil {
                panic("")
            }            
        }
        for {
            data = <- for_process
            _, err = writeStringToPipe(data.word, mystem_writer)
            answer.err = err
            if config.format != "xml" {
                _, err = readStringFromPipe(&answer.data, mystem_reader)
                if err != nil {
                    answer.err = err
                    break
                }
            } else {
                for answer.data == "" {
                    _, err = readStringFromPipe(&answer.data, mystem_reader)
                    if err != nil {
                        answer.err = err
                        break
                    }
                }
            }
            data.channel <- answer
            answer.data = ""
            answer.err = nil
        }
    }
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
    //resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
    if words_count > 0 {
        var err error
        var local_channel chan Answer = make(chan Answer, words_count)
        data.channel = local_channel
        var start int = 0
        for _, word := range words {
            err = nil
            if reg_filter.MatchString(word) {
                data.word = word
                for_process <- data
            } else {
                start++
                switch config.format {
                    case "json": {
                        tmp_answer = fmt.Sprintf(`{"analysis":[], "text":"%v"}`, word)
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
            if answer.err != nil {
                err = answer.err
            }
            answers = append(answers, answer.data)
        }
        if err == nil {
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
        } else {
            //request_answer = fmt.Sprintf(`{"status": %v, "reason": "%v"}`, status_code, http.StatusText(http.StatusInternalServerError)
            resp.WriteHeader(http.StatusInternalServerError)
        }
        close(local_channel)
    } else {
        //request_answer = fmt.Sprintf(`{"status": %v, "reason": "%v"}`, status_code, http.StatusText(status_code))
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