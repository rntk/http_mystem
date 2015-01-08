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
    "math/rand"
)

type Data struct {
    word string
    answer string
    resp http.ResponseWriter
}

var for_process = make(chan Data, 10)
var for_response = make(chan Data, 10)

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

func workerMystem(for_process chan Data, for_response chan Data, mystem_path string) int {
    var data Data
    var buf []byte
    name := rand.Int()
    log.Print("Mystem started ", name)
    mystem := exec.Command(mystem_path, "-n")
    mystem_writer, err_w := mystem.StdinPipe()
    mystem_reader, err_r := mystem.StdoutPipe()
    err := mystem.Start()
    var msg string
    if (err_w != nil) || (err_r != nil) {
        msg := fmt.Sprintf("Can't start: \n%v\n%v", err_w, err_r)
        log.Fatal(msg)
        panic(msg)
    }
    if err != nil {
        msg = fmt.Sprintf("Can't start: %v", err)
        log.Fatal(msg)
        panic(msg)
    } else {
        var n int
        for {
            data = <- for_process
            buf = make([]byte, 1000)
            n, err = mystem_writer.Write([]byte(fmt.Sprintf("%v\n", data.word)))
            if err != nil {
                msg = fmt.Sprintf("Can't send word to mystem: %v", err)
                log.Fatal(msg)
                panic(msg)
            }
            n, err = mystem_reader.Read(buf)
            if err != nil {
                msg = fmt.Sprintf("Can't read answer from mystem: %v", err)
                log.Fatal(msg)
                panic(msg)
            }
            data.answer = string(buf[:n])
            for_response <- data
            time.Sleep(time.Millisecond * 100)
        }
    }
    return 0
}

func workerResponse(channel chan Data) int {
    var data Data
    name := rand.Int()
    log.Print("Responser started ", name)
    for {
        data = <- channel
        data.resp.Write([]byte(data.answer))
        time.Sleep(time.Second)
    }
    return 0
}

func processRequest(resp http.ResponseWriter, req *http.Request) {
    var data Data
    req.ParseForm()
    if len(req.Form["word"]) > 0 {
        word := req.Form["word"][0]
        data.word = word
        data.resp = resp
        for_process <- data
    } else {
        resp.Write([]byte("Word can't be empty"))
    }
}

func main() {
    config, err := loadConfig()
    if err != nil {
        msg := fmt.Sprintf("Can't load config: %v", err)
        log.Fatal(msg)
        panic(msg)
    } else {
        log.Print("Config load succesful")
        mystem_workers, err := strconv.ParseUint(config["mystem_workers"], 10, 8)
        response_workers, err := strconv.ParseUint(config["response_workers"], 10, 8)
        msg := ""
        if err != nil {
            msg = fmt.Sprintf("Can't get mystem_workers: %v", err)
            log.Fatal(msg)
            panic(msg)
        }
        if err != nil {
            msg = fmt.Sprintf("Can't get response_workers: %v", err)
            log.Fatal(msg)
            panic(msg)
        }
        _, err = os.Open(config["mystem_path"])
        if  err != nil {
            msg = fmt.Sprintf("Can't find mystem: %v", err)
            log.Fatal(msg)
            panic(msg)
        }
        var i uint64
        for i = 0; i < mystem_workers; i++ {
            go workerMystem(for_process, for_response, config["mystem_path"])
        }
        for i = 0; i < response_workers; i++ {
            go workerResponse(for_response)
        }
        http.HandleFunc("/", processRequest)
        err = http.ListenAndServe(fmt.Sprintf("%v:%v", config["host"], config["port"]), nil)
        if err != nil {
            log.Print("Can't start http server: ", err)
        } else {
            log.Print(fmt.Sprintf("Server start on: %v:%v", config["host"], config["port"]))
        }
    }
}