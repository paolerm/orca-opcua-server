/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpcuaServerSpec defines the desired state of OpcuaServer
type OpcuaServerSpec struct {
	// Id
	Id string `json:"id"`
	// Number of Servers
	ServerCount int `json:"serverCount"`
	// Number of Assets for each server
	AssetPerServer int `json:"assetPerServer"`
	// Number of tags for each server
	TagCount int `json:"tagCount"`
	// Asset update rate per second
	AssetUpdateRatePerSecond int `json:"assetUpdateRatePerSecond"`
	// Rate in MS to change nodes
	ChangeRateMs int `json:"changeRateMs"`
	// Node sampling interval in MS
	SamplingIntervalMs int `json:"samplingIntervalMs"`
	// Docker image ID to use (if not defined, uses default)
	DockerImageId string `json:"dockerImageId,omitempty"`
	// Log level
	LogLevel string `json:"logLevel,omitempty"`
	// OPCUA server log level
	OpcuaServerLogLevel string `json:"opcuaServerLogLevel,omitempty"`
}

// OpcuaServerStatus defines the observed state of OpcuaServer
type OpcuaServerStatus struct {
	// IP address that exposes all the OPCUA discovery endpoints for each server
	PublicIpAddress []string `json:"publicIpAddress"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OpcuaServer is the Schema for the opcuaservers API
type OpcuaServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpcuaServerSpec   `json:"spec,omitempty"`
	Status OpcuaServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpcuaServerList contains a list of OpcuaServer
type OpcuaServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpcuaServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpcuaServer{}, &OpcuaServerList{})
}
