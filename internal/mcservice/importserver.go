package mcservice

import (
    "encoding/base64"
    "encoding/json"
    "io"
    "net"
    "net/http"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/content"
    "github.com/liteldev/LeviLauncher/internal/mods"
)

func StartImportServer() (*http.Server, net.Listener, string) {
    mux := http.NewServeMux()
    mux.HandleFunc("/_ping", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    mux.HandleFunc("/api/import/modzip", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            _, _ = w.Write([]byte(`{"error":"METHOD_NOT_ALLOWED"}`))
            return
        }
        var name string
        var overwrite bool
        var data []byte
        ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
        if strings.HasPrefix(ct, "multipart/form-data") {
            _ = r.ParseMultipartForm(64 << 20)
            name = strings.TrimSpace(r.FormValue("name"))
            ow := strings.TrimSpace(r.FormValue("overwrite"))
            if ow != "" {
                l := strings.ToLower(ow)
                overwrite = l == "1" || l == "true" || l == "yes"
            }
            f, fh, err := r.FormFile("file")
            if err == nil && f != nil {
                defer f.Close()
                b, er := io.ReadAll(f)
                if er == nil {
                    data = b
                }
            }
            if len(data) == 0 && fh != nil {
                _ = f.Close()
            }
        } else {
            b, _ := io.ReadAll(r.Body)
            _ = r.Body.Close()
            var obj map[string]interface{}
            if err := json.Unmarshal(b, &obj); err == nil {
                if v, ok := obj["name"].(string); ok {
                    name = strings.TrimSpace(v)
                }
                if v, ok := obj["overwrite"].(bool); ok {
                    overwrite = v
                } else if v2, ok2 := obj["overwrite"].(string); ok2 {
                    l := strings.ToLower(strings.TrimSpace(v2))
                    overwrite = l == "1" || l == "true" || l == "yes"
                }
                if v, ok := obj["data"].(string); ok && v != "" {
                    bs, _ := base64.StdEncoding.DecodeString(v)
                    if len(bs) > 0 {
                        data = bs
                    }
                }
            }
        }
        if name == "" || len(data) == 0 {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte(`{"error":"BAD_REQUEST"}`))
            return
        }
        err := mods.ImportZipToMods(name, data, overwrite)
        if err != "" {
            _, _ = w.Write([]byte(`{"error":"` + err + `"}`))
            return
        }
        _, _ = w.Write([]byte(`{"error":""}`))
    })
    mux.HandleFunc("/api/import/moddll", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            _, _ = w.Write([]byte(`{"error":"METHOD_NOT_ALLOWED"}`))
            return
        }
        var name, fileName, modName, modType, version string
        var overwrite bool
        var data []byte
        ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
        if strings.HasPrefix(ct, "multipart/form-data") {
            _ = r.ParseMultipartForm(64 << 20)
            name = strings.TrimSpace(r.FormValue("name"))
            fileName = strings.TrimSpace(r.FormValue("fileName"))
            modName = strings.TrimSpace(r.FormValue("modName"))
            modType = strings.TrimSpace(r.FormValue("modType"))
            version = strings.TrimSpace(r.FormValue("version"))
            ow := strings.TrimSpace(r.FormValue("overwrite"))
            if ow != "" {
                l := strings.ToLower(ow)
                overwrite = l == "1" || l == "true" || l == "yes"
            }
            f, fh, err := r.FormFile("file")
            if err == nil && f != nil {
                defer f.Close()
                b, er := io.ReadAll(f)
                if er == nil {
                    data = b
                }
                if fileName == "" && fh != nil {
                    fileName = fh.Filename
                }
            }
        } else {
            b, _ := io.ReadAll(r.Body)
            _ = r.Body.Close()
            var obj map[string]interface{}
            if err := json.Unmarshal(b, &obj); err == nil {
                if v, ok := obj["name"].(string); ok {
                    name = strings.TrimSpace(v)
                }
                if v, ok := obj["fileName"].(string); ok {
                    fileName = strings.TrimSpace(v)
                }
                if v, ok := obj["modName"].(string); ok {
                    modName = strings.TrimSpace(v)
                }
                if v, ok := obj["modType"].(string); ok {
                    modType = strings.TrimSpace(v)
                }
                if v, ok := obj["version"].(string); ok {
                    version = strings.TrimSpace(v)
                }
                if v, ok := obj["overwrite"].(bool); ok {
                    overwrite = v
                } else if v2, ok2 := obj["overwrite"].(string); ok2 {
                    l := strings.ToLower(strings.TrimSpace(v2))
                    overwrite = l == "1" || l == "true" || l == "yes"
                }
                if v, ok := obj["data"].(string); ok && v != "" {
                    bs, _ := base64.StdEncoding.DecodeString(v)
                    if len(bs) > 0 {
                        data = bs
                    }
                }
            }
        }
        if name == "" || modName == "" || modType == "" || version == "" || len(data) == 0 {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte(`{"error":"BAD_REQUEST"}`))
            return
        }
        err := mods.ImportDllToMods(name, fileName, data, modName, modType, version, overwrite)
        if err != "" {
            _, _ = w.Write([]byte(`{"error":"` + err + `"}`))
            return
        }
        _, _ = w.Write([]byte(`{"error":""}`))
    })
    mux.HandleFunc("/api/import/mcpack", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            _, _ = w.Write([]byte(`{"error":"METHOD_NOT_ALLOWED"}`))
            return
        }
        var name string
        var overwrite bool
        var data []byte
        var fileName string
        var player string
        ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
        if strings.HasPrefix(ct, "multipart/form-data") {
            _ = r.ParseMultipartForm(64 << 20)
            name = strings.TrimSpace(r.FormValue("name"))
            player = strings.TrimSpace(r.FormValue("player"))
            ow := strings.TrimSpace(r.FormValue("overwrite"))
            if ow != "" {
                l := strings.ToLower(ow)
                overwrite = l == "1" || l == "true" || l == "yes"
            }
            f, fh, err := r.FormFile("file")
            if err == nil && f != nil {
                defer f.Close()
                b, er := io.ReadAll(f)
                if er == nil {
                    data = b
                }
                if fh != nil {
                    fileName = fh.Filename
                }
            }
        } else {
            b, _ := io.ReadAll(r.Body)
            _ = r.Body.Close()
            var obj map[string]interface{}
            if err := json.Unmarshal(b, &obj); err == nil {
                if v, ok := obj["name"].(string); ok {
                    name = strings.TrimSpace(v)
                }
                if v, ok := obj["overwrite"].(bool); ok {
                    overwrite = v
                } else if v2, ok2 := obj["overwrite"].(string); ok2 {
                    l := strings.ToLower(strings.TrimSpace(v2))
                    overwrite = l == "1" || l == "true" || l == "yes"
                }
                if v, ok := obj["fileName"].(string); ok {
                    fileName = strings.TrimSpace(v)
                }
                if v, ok := obj["player"].(string); ok {
                    player = strings.TrimSpace(v)
                }
                if v, ok := obj["data"].(string); ok && v != "" {
                    bs, _ := base64.StdEncoding.DecodeString(v)
                    if len(bs) > 0 {
                        data = bs
                    }
                }
            }
        }
        if name == "" || len(data) == 0 {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte(`{"error":"BAD_REQUEST"}`))
            return
        }
        roots := GetContentRoots(name)
        skinDir := ""
        if strings.TrimSpace(player) != "" && strings.TrimSpace(roots.UsersRoot) != "" {
            skinDir = filepath.Join(roots.UsersRoot, player, "games", "com.mojang", "skin_packs")
        }
        err := content.ImportMcpackToDirs2(data, fileName, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
        if err != "" {
            _, _ = w.Write([]byte(`{"error":"` + err + `"}`))
            return
        }
        _, _ = w.Write([]byte(`{"error":""}`))
    })
    mux.HandleFunc("/api/import/mcaddon", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            _, _ = w.Write([]byte(`{"error":"METHOD_NOT_ALLOWED"}`))
            return
        }
        var name string
        var overwrite bool
        var data []byte
        var player string
        ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
        if strings.HasPrefix(ct, "multipart/form-data") {
            _ = r.ParseMultipartForm(64 << 20)
            name = strings.TrimSpace(r.FormValue("name"))
            player = strings.TrimSpace(r.FormValue("player"))
            ow := strings.TrimSpace(r.FormValue("overwrite"))
            if ow != "" {
                l := strings.ToLower(ow)
                overwrite = l == "1" || l == "true" || l == "yes"
            }
            f, _, err := r.FormFile("file")
            if err == nil && f != nil {
                defer f.Close()
                b, er := io.ReadAll(f)
                if er == nil {
                    data = b
                }
            }
        } else {
            b, _ := io.ReadAll(r.Body)
            _ = r.Body.Close()
            var obj map[string]interface{}
            if err := json.Unmarshal(b, &obj); err == nil {
                if v, ok := obj["name"].(string); ok {
                    name = strings.TrimSpace(v)
                }
                if v, ok := obj["overwrite"].(bool); ok {
                    overwrite = v
                } else if v2, ok2 := obj["overwrite"].(string); ok2 {
                    l := strings.ToLower(strings.TrimSpace(v2))
                    overwrite = l == "1" || l == "true" || l == "yes"
                }
                if v, ok := obj["player"].(string); ok {
                    player = strings.TrimSpace(v)
                }
                if v, ok := obj["data"].(string); ok && v != "" {
                    bs, _ := base64.StdEncoding.DecodeString(v)
                    if len(bs) > 0 {
                        data = bs
                    }
                }
            }
        }
        if name == "" || len(data) == 0 {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte(`{"error":"BAD_REQUEST"}`))
            return
        }
        roots := GetContentRoots(name)
        skinDir := ""
        if strings.TrimSpace(player) != "" && strings.TrimSpace(roots.UsersRoot) != "" {
            skinDir = filepath.Join(roots.UsersRoot, player, "games", "com.mojang", "skin_packs")
        }
        err := content.ImportMcaddonToDirs2(data, roots.ResourcePacks, roots.BehaviorPacks, skinDir, overwrite)
        if err != "" {
            _, _ = w.Write([]byte(`{"error":"` + err + `"}`))
            return
        }
        _, _ = w.Write([]byte(`{"error":""}`))
    })
    mux.HandleFunc("/api/import/mcworld", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "*")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            _, _ = w.Write([]byte(`{"error":"METHOD_NOT_ALLOWED"}`))
            return
        }
        var name, player, fileName string
        var overwrite bool
        var data []byte
        ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
        if strings.HasPrefix(ct, "multipart/form-data") {
            _ = r.ParseMultipartForm(64 << 20)
            name = strings.TrimSpace(r.FormValue("name"))
            player = strings.TrimSpace(r.FormValue("player"))
            fileName = strings.TrimSpace(r.FormValue("fileName"))
            ow := strings.TrimSpace(r.FormValue("overwrite"))
            if ow != "" {
                l := strings.ToLower(ow)
                overwrite = l == "1" || l == "true" || l == "yes"
            }
            f, fh, err := r.FormFile("file")
            if err == nil && f != nil {
                defer f.Close()
                b, er := io.ReadAll(f)
                if er == nil {
                    data = b
                }
                if fileName == "" && fh != nil {
                    fileName = fh.Filename
                }
            }
        } else {
            b, _ := io.ReadAll(r.Body)
            _ = r.Body.Close()
            var obj map[string]interface{}
            if err := json.Unmarshal(b, &obj); err == nil {
                if v, ok := obj["name"].(string); ok {
                    name = strings.TrimSpace(v)
                }
                if v, ok := obj["player"].(string); ok {
                    player = strings.TrimSpace(v)
                }
                if v, ok := obj["fileName"].(string); ok {
                    fileName = strings.TrimSpace(v)
                }
                if v, ok := obj["overwrite"].(bool); ok {
                    overwrite = v
                } else if v2, ok2 := obj["overwrite"].(string); ok2 {
                    l := strings.ToLower(strings.TrimSpace(v2))
                    overwrite = l == "1" || l == "true" || l == "yes"
                }
                if v, ok := obj["data"].(string); ok && v != "" {
                    bs, _ := base64.StdEncoding.DecodeString(v)
                    if len(bs) > 0 {
                        data = bs
                    }
                }
            }
        }
        if name == "" || player == "" || len(data) == 0 {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte(`{"error":"BAD_REQUEST"}`))
            return
        }
        err := ImportMcworld(name, player, fileName, data, overwrite)
        if err != "" {
            _, _ = w.Write([]byte(`{"error":"` + err + `"}`))
            return
        }
        _, _ = w.Write([]byte(`{"error":""}`))
    })
    srv := &http.Server{Handler: mux}
    addrs := []int{32773, 32774, 32775, 32776, 32777}
    var ln net.Listener
    var addrStr string
    for _, p := range addrs {
        l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
        if err == nil {
            ln = l
            addrStr = "http://127.0.0.1:" + strconv.Itoa(p)
            break
        }
    }
    if ln == nil {
        l, err := net.Listen("tcp", "127.0.0.1:0")
        if err != nil {
            return nil, nil, ""
        }
        ln = l
        addr := ln.Addr().String()
        if strings.HasPrefix(addr, "[::]") {
            addr = strings.Replace(addr, "[::]", "127.0.0.1", 1)
        }
        addrStr = "http://" + addr
    }
    return srv, ln, addrStr
}