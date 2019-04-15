# Projector Backend

Backend API that implements [Impress Remote Protocol](https://wiki.documentfoundation.org/Development/Impress_Remote_Protocol) and serves as a common gateway for multiple users to connect and control the same impress presentation. Communication with impress happens through a **socket** and the messages arriving from users are **queued**, while communication with users happens through **web socket** and the messages from impress are **broadcasted**. It also features a couple of **HTTP endpoints** that are usefull for checking running presentation status and room availability.

Since the project was designed to be an **IoT project** it contains a module that generates a fresh QR code at every startup. This QR Code encodes the current WiFi access details that are available in the running configuration. The device running this server should be configuread as an **WiFi access point**.

Files from `qr-directory` and `client-directory` are served statically at `/qr` and `/client` respectively.

## Requirements
* Docker for build
* Impress instance [configured](https://opensourceforu.com/2016/02/impress-remote-an-android-app-for-libreoffice-presentations/) to accept remote connections and has also given access to this server as a remote controller
* Client web application to be served at /client
* Device configured as WiFi access point with all network traffic routed to client's url in order to trigger captive portal launching on mobile devices

## Build
Run `build.sh` to build the project. The executable will be avaiable in project's root directory.
*Notes:* In order to build, **docker** is required. You can configure the target build os and architecture in `Dockerfile` by changing the values of `GOOS` and `GOARCH`


## Configuration

Configuration can be given via **executable flags**, **environment variables**, or **HCL config file**, named *aplication.conf*. The priority of any given configuration comes in the same order. Below is a table with the available configuration keys and their descriptions.

Key | Description
------------ | -------------
libre-office-path | Executable path for Libre Office
libre-remote-url | The URL for libre remote connection
libre-remote-name | Name of the remote controller [This server]
libre-remote-pin  | The PIN for the remote controller [This server]
libre-max-controllers | The maximum number of user connections to the presentation
libre-max-timeout | The maximum number of seconds the presentation owner is allowed to be disconnected before presentation drop 
max-upoad-size | The maximum upload size in bytes for the uploaded presentations
uploads-directory  | The folder that temporary host the uploaded presentations
qr-directory | The directory from where the QR website is served
client-directory | The directory from where the client web application is served
network-ssid | The network SSID. Used to generate the QR Code
network-pass | The network password. Used to generate the QR Code
http-addr | Address for the http server

## Run
1. Configure the device as a WiFi access point with routed network traffic
2. Build the application for the desired operating system and architecture
3. Adapt the configuration 
4. Copy the client web application's build inside `client-directory`
5. Run the executable