// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_luaDir      = "./lua"
	_namespace   = "istio-system"
	_mountPrefix = "/usr/local/share/lua/5.1"
)

type configVolume struct {
	configMap map[string]string
	mountPath string
	name      string
}

type userConfigMap struct {
	Name string `json:"name"`
}

type userVolume struct {
	ConfigMap userConfigMap `json:"configMap"`
	Name      string        `json:"name"`
}

type userVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

func saveConfigMap(name string, cm *corev1.ConfigMap) {
	value, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		panic(err)
	}
	path := filepath.Join("configmaps", name)
	if err := ioutil.WriteFile(path, value, 0644); err != nil {
		panic(err)
	}
	fmt.Println("Created configmap file", path)
}

func saveHelmSetFlags(builder *strings.Builder, i int, mountPath, name, configMapName string) {
	value := fmt.Sprintf("--set gateways.istio-ingressgateway.configVolumes\\[%d\\].mountPath=\"%s\" \\ \n", i, mountPath)
	if _, err := builder.WriteString(value); err != nil {
		panic(err)
	}
	value = fmt.Sprintf("--set gateways.istio-ingressgateway.configVolumes\\[%d\\].name=\"%s\" \\ \n", i, name)
	if _, err := builder.WriteString(value); err != nil {
		panic(err)
	}
	value = fmt.Sprintf("--set gateways.istio-ingressgateway.configVolumes\\[%d\\].configMapName=\"%s\" \\ \n", i, configMapName)
	if _, err := builder.WriteString(value); err != nil {
		panic(err)
	}
}

func main() {
	if v := os.Getenv("LUA_DIR"); v != "" {
		_luaDir = v
	}
	if v := os.Getenv("NAMESPACE"); v != "" {
		_namespace = v
	}
	if v := os.Getenv("MOUNT_PREFIX"); v != "" {
		_mountPrefix = v
	}
	if err := os.Remove("helmset"); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.Remove("kustomization.yaml"); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.RemoveAll("configmaps"); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.Mkdir("configmaps", 0755); err != nil {
		panic(err)
	}

	var (
		cnt     int
		res     []string
		builder strings.Builder
	)
	suiteset := make(map[string]*configVolume)
	uv := make(map[string]*userVolume)
	uvm := make(map[string]*userVolumeMount)

	err := filepath.Walk(_luaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".lua") {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				panic(err)
			}
			dir := filepath.Dir(filepath.Join(_mountPrefix, strings.TrimPrefix(path, _luaDir)))
			suite, ok := suiteset[dir]
			if !ok {
				name := fmt.Sprintf("envoy-apisix-configmap-%d", cnt)
				suite = &configVolume{
					configMap: make(map[string]string),
					mountPath: dir,
					name:      name,
				}
				suiteset[dir] = suite
				cnt++
				uv[name] = &userVolume{
					ConfigMap: userConfigMap{
						Name: name,
					},
					Name: name,
				}
				uvm[name] = &userVolumeMount{
					Name:      name,
					MountPath: dir,
				}
			}
			//suite.configMap[info.Name()] = strconv.Quote(string(content))
			suite.configMap[info.Name()] = string(content)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	cnt = 0
	for mountPath, suite := range suiteset {
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: suite.name,
			},
			Data: suite.configMap,
		}
		saveConfigMap(suite.name, cm)
		saveHelmSetFlags(&builder, cnt, mountPath, suite.name, suite.name)
		cnt++
		res = append(res, suite.name)
	}

	buf := bytes.NewBuffer(nil)
	if _, err := buf.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\n"); err != nil {
		panic(err)
	}
	if _, err := buf.WriteString("kind: Kustomization\n"); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(buf, "namespace: %s\n", _namespace); err != nil {
		panic(err)
	}
	if _, err := buf.WriteString("commonLabels:\n  apisix.apache.org/created_by: envoy-apisix\n"); err != nil {
		panic(err)
	}
	if _, err := buf.WriteString("resources:\n"); err != nil {
		panic(err)
	}
	for _, r := range res {
		if _, err := fmt.Fprintf(buf, "  - ./configmaps/%s\n", r); err != nil {
			panic(err)
		}
	}
	if err := ioutil.WriteFile("kustomization.yaml", buf.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Created kustomization.yaml")
	fmt.Println("\nRun\n\tkubectl apply -k .\n\nto install configmaps in namespace", _namespace)

	if _namespace == "istio-system" {
		fmt.Println("\nPlease add these flags when you use helm to install istiod/istio-ingressgateway\n")
		fmt.Println(builder.String())
		return
	}

	fmt.Println("\nPlease add the following annotations to your application Pod template\n")

	value, err := json.Marshal(uv)
	if err != nil {
		panic(err)
	}
	fmt.Println("sidecar.istio.io/userVolume: |")
	fmt.Println("  ", string(value), "\n")

	value, err = json.Marshal(uvm)
	if err != nil {
		panic(err)
	}
	fmt.Println("sidecar.istio.io/userVolumeMount: |")
	fmt.Println("  ", string(value))
}
