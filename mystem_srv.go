package main

import (
    "fmt"
    "net/http"
    "os"
    "os/exec"
    "encoding/json"
    "log"
    "time"
    "strconv"
    "strings"
    "math/rand"
)

type Data struct {
    word string
    channel chan string
}

var for_process chan Data

func makePanic(msg string) {
    log.Fatal(msg)
    panic(msg)    
}

func loadConfig() (config map[string]string, err error) {
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
            err = json.Unmarshal(raw_json, &config)
        }
    }
    return config, err
}

func workerMystem(for_process chan Data, mystem_path string) int {
    name := rand.Int()
    log.Print("Mystem started ", name)
    mystem := exec.Command(mystem_path, "-n")
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
        var answer string
        for {
            data = <- for_process
            fmt.Printf("Worker %v write %v\n", name, data.word)
            n, err = mystem_writer.Write([]byte(fmt.Sprintf("%v\n", data.word)))
            fmt.Printf("Worker %v writed %v\n", name, data.word)
            if err != nil {
                makePanic(fmt.Sprintf("Can't send word to mystem: %v", err))
            }
            fmt.Printf("Worker %v try read\n", name)
            buf = make([]byte, 1000)
            n, err = mystem_reader.Read(buf)
            if err != nil {
                makePanic(fmt.Sprintf("Can't read answer from mystem: %v", err))
            }
            answer = strings.TrimSpace(string(buf[:n]))
            fmt.Printf("Worker %v say %v\n", name, answer)
            //time.Sleep(time.Second * time.Duration(rand.Intn(5)))
            data.channel <- answer
            time.Sleep(time.Millisecond * 100)
        }
    }
    return 0
}

func processWords(resp http.ResponseWriter, req *http.Request) {
    var data Data
    var answer string = ""
    var request_answer string = "["
    req.ParseForm()
    var words_count = len(req.Form["words"])
    var local_channel = make(chan string, words_count)
    if words_count > 0 {
        data.channel = local_channel
        for _, word := range req.Form["words"] {
            data.word = word
            for_process <- data
        }
        for i := 0; i < words_count; i++ {
            answer = <- local_channel
            if (i + 1) < words_count {
                answer += ","
            }
            request_answer += answer
        }
        request_answer += "]"
        resp.Write([]byte(request_answer))
    } else {
        resp.Write([]byte("Word can't be empty"))
    }
}

func main() {
    config, err := loadConfig()
    if err != nil {
        makePanic(fmt.Sprintf("Can't load config: %v", err))
    } else {
        mystem_workers, err := strconv.ParseUint(config["mystem_workers"], 10, 8)
        if err != nil {
            makePanic(fmt.Sprintf("Can't get mystem_workers: %v", err))
        }
        _, err = os.Open(config["mystem_path"])
        if  err != nil {
            makePanic(fmt.Sprintf("Can't find mystem: %v", err))
        }
        var i uint64
        var buf_size uint64
        buf_size, err = strconv.ParseUint(config["channel_buffer"], 10, 64)
        if err != nil {
            makePanic(fmt.Sprintf("Can't process channel_buffer: %v", err))
        }
        if buf_size > 0 {
            for_process = make(chan Data, buf_size)
        } else {
            for_process = make(chan Data)
        }
        log.Print("Config load successful")
        for i = 0; i < mystem_workers; i++ {
            go workerMystem(for_process, config["mystem_path"])
        }
        http.HandleFunc("/", processWords)
        log.Print(fmt.Sprintf("Server start on: %v:%v", config["host"], config["port"]))
        err = http.ListenAndServe(fmt.Sprintf("%v:%v", config["host"], config["port"]), nil)
        if err != nil {
            makePanic(fmt.Sprintf("Can't start http server: %v", err))
        }
    }
}

/*TODO
url /word for one word
url /words for many words (word1,word2, ..., wordN)
web ui
mystem options from conf.json
*/