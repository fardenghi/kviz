package api_test

// Env, ConfigMaps, Secrets y volúmenes en la foto (ticket 10): el origen de cada variable de
// entorno, el contenido de los ConfigMaps, los Secrets sin valores y los volúmenes con sus
// montajes. Los casos salen de la guía: 5a-nginx.yaml, 5b-postgres.yaml y 6b-deployment.yaml.
//
// Los Secrets de los fixtures llevan valores falsos: nunca la contraseña real de la guía.

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Contrato JSON de env, montajes y volúmenes (web/src/foto.ts).
type variableEnFoto struct {
	Nombre  string  `json:"nombre"`
	Origen  string  `json:"origen"`
	Fuente  string  `json:"fuente"`
	Clave   string  `json:"clave"`
	Valor   *string `json:"valor"`
	EnvFrom bool    `json:"envFrom"`
	Falta   bool    `json:"falta"`
}

type montajeEnFoto struct {
	Volumen     string `json:"volumen"`
	Ruta        string `json:"ruta"`
	SoloLectura bool   `json:"soloLectura"`
	SubPath     string `json:"subPath"`
}

type volumenEnFoto struct {
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
	Fuente string `json:"fuente"`
}

type containerConEnv struct {
	Nombre   string           `json:"nombre"`
	Env      []variableEnFoto `json:"env"`
	Montajes []montajeEnFoto  `json:"montajes"`
}

type podConEnv struct {
	Nombre     string            `json:"nombre"`
	Containers []containerConEnv `json:"containers"`
	Volumenes  []volumenEnFoto   `json:"volumenes"`
}

func (k *kvizDePrueba) podConEnv(t *testing.T, nombre string) podConEnv {
	t.Helper()
	var f struct {
		Pods []podConEnv `json:"pods"`
	}
	k.getJSON(t, "/api/foto", &f)
	for _, p := range f.Pods {
		if p.Nombre == nombre {
			return p
		}
	}
	t.Fatalf("no está el pod %s en la foto: %s", nombre, mostrar(f.Pods))
	return podConEnv{}
}

func TestNginxTomaSuEnvDeUnConfigMapConEnvFrom(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nginxDeLaGuia()...)

	got := kviz.podConEnv(t, "nginx").Containers[0].Env

	want := []variableEnFoto{
		{Nombre: "NGINX_PORT", Origen: "ConfigMap", Fuente: "nginx-env", Clave: "NGINX_PORT", Valor: ptr("80"), EnvFrom: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("env = %s\nwant %s", mostrar(got), mostrar(want))
	}
}

func TestNginxMontaUnConfigMapComoVolumen(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nginxDeLaGuia()...)

	p := kviz.podConEnv(t, "nginx")

	wantVolumenes := []volumenEnFoto{{Nombre: "templates", Tipo: "ConfigMap", Fuente: "nginx-template"}}
	if !reflect.DeepEqual(p.Volumenes, wantVolumenes) {
		t.Errorf("volúmenes = %s\nwant %s", mostrar(p.Volumenes), mostrar(wantVolumenes))
	}
	wantMontajes := []montajeEnFoto{{Volumen: "templates", Ruta: "/etc/nginx/templates", SoloLectura: true}}
	if got := p.Containers[0].Montajes; !reflect.DeepEqual(got, wantMontajes) {
		t.Errorf("montajes = %s\nwant %s", mostrar(got), mostrar(wantMontajes))
	}
}

type configMapEnFoto struct {
	ID        string            `json:"id"`
	Nombre    string            `json:"nombre"`
	Namespace string            `json:"namespace"`
	Datos     map[string]string `json:"datos"`
}

func TestLosConfigMapsSeVenConSuContenido(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nginxDeLaGuia()...)

	var f struct {
		ConfigMaps []configMapEnFoto `json:"configMaps"`
	}
	kviz.getJSON(t, "/api/foto", &f)

	want := []configMapEnFoto{
		{ID: "default/nginx-env", Nombre: "nginx-env", Namespace: "default", Datos: map[string]string{"NGINX_PORT": "80"}},
		{ID: "default/nginx-template", Nombre: "nginx-template", Namespace: "default", Datos: map[string]string{"default.conf.template": plantillaNginx}},
	}
	if !reflect.DeepEqual(f.ConfigMaps, want) {
		t.Errorf("configMaps = %s\nwant %s", mostrar(f.ConfigMaps), mostrar(want))
	}
}

type secretEnFoto struct {
	ID        string   `json:"id"`
	Nombre    string   `json:"nombre"`
	Namespace string   `json:"namespace"`
	Tipo      string   `json:"tipo"`
	Claves    []string `json:"claves"`
}

func TestPostgresTomaSuEnvDeUnSecretSinElValor(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	got := kviz.podConEnv(t, "postgres").Containers[0].Env

	want := []variableEnFoto{
		{Nombre: "POSTGRES_PASSWORD", Origen: "Secret", Fuente: "postgres-secret", Clave: "POSTGRES_PASSWORD", Valor: nil, EnvFrom: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("env = %s\nwant %s", mostrar(got), mostrar(want))
	}
}

func TestLosSecretsVienenSoloConSusClaves(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	var f struct {
		Secrets []secretEnFoto `json:"secrets"`
	}
	kviz.getJSON(t, "/api/foto", &f)

	want := []secretEnFoto{{ID: "default/postgres-secret", Nombre: "postgres-secret", Namespace: "default", Tipo: "Opaque", Claves: []string{"POSTGRES_PASSWORD"}}}
	if !reflect.DeepEqual(f.Secrets, want) {
		t.Errorf("secrets = %s\nwant %s", mostrar(f.Secrets), mostrar(want))
	}
}

func TestLosPodsDelDeploymentBackendTomanNAMEDeSuMetadataName(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", backendDeLaGuia()...)

	for _, nombre := range []string{"backend-79f9ffc666-k2x8p", "backend-79f9ffc666-7hw4q"} {
		got := kviz.podConEnv(t, nombre).Containers[0].Env
		want := []variableEnFoto{
			{Nombre: "SERVER_PORT", Origen: "valor", Valor: ptr("8081")},
			{Nombre: "NAME", Origen: "DownwardAPI", Fuente: "metadata.name", Valor: ptr(nombre)},
			{Nombre: "VERSION", Origen: "valor", Valor: ptr("v1")},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("env de %s = %s\nwant %s", nombre, mostrar(got), mostrar(want))
		}
	}
}

func TestCadaVariableTraeSuOrigenYSuFuente(t *testing.T) {
	p := pod("default", "api", "lab-worker", "10.244.1.9", corev1.PodRunning)
	ref := func(nombre string) corev1.LocalObjectReference { return corev1.LocalObjectReference{Name: nombre} }
	p.Spec.Containers = []corev1.Container{{
		Name: "api",
		EnvFrom: []corev1.EnvFromSource{
			{Prefix: "DB_", SecretRef: &corev1.SecretEnvSource{LocalObjectReference: ref("db")}},
			{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: ref("no-existe")}},
			{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: ref("api-config")}},
		},
		Env: []corev1.EnvVar{
			// pisa al LOG_LEVEL del envFrom: el proceso ve este
			{Name: "LOG_LEVEL", Value: "debug"},
			{Name: "REGION", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: ref("api-config"), Key: "region"}}},
			{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: ref("db"), Key: "token"}}},
			{Name: "FALTA", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: ref("api-config"), Key: "no-hay"}}},
			{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
			{Name: "CPU", ValueFrom: &corev1.EnvVarSource{ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: "limits.cpu"}}},
		},
	}}
	kviz := nuevoKviz(t, "kind-lab", p,
		configMap("default", "api-config", map[string]string{"LOG_LEVEL": "info", "region": "sa-east-1"}),
		secret("default", "db", map[string]string{"user": "valor-falso-1", "token": "valor-falso-2"}),
		// mismo nombre en otro namespace: no es la fuente del pod
		configMap("otro", "no-existe", map[string]string{"X": "1"}),
	)

	got := kviz.podConEnv(t, "api").Containers[0].Env

	want := []variableEnFoto{
		{Nombre: "DB_token", Origen: "Secret", Fuente: "db", Clave: "token", EnvFrom: true},
		{Nombre: "DB_user", Origen: "Secret", Fuente: "db", Clave: "user", EnvFrom: true},
		{Nombre: "", Origen: "ConfigMap", Fuente: "no-existe", EnvFrom: true, Falta: true},
		{Nombre: "LOG_LEVEL", Origen: "valor", Valor: ptr("debug")},
		{Nombre: "region", Origen: "ConfigMap", Fuente: "api-config", Clave: "region", Valor: ptr("sa-east-1"), EnvFrom: true},
		{Nombre: "REGION", Origen: "ConfigMap", Fuente: "api-config", Clave: "region", Valor: ptr("sa-east-1")},
		{Nombre: "TOKEN", Origen: "Secret", Fuente: "db", Clave: "token"},
		{Nombre: "FALTA", Origen: "ConfigMap", Fuente: "api-config", Clave: "no-hay", Falta: true},
		{Nombre: "POD_IP", Origen: "DownwardAPI", Fuente: "status.podIP", Valor: ptr("10.244.1.9")},
		{Nombre: "CPU", Origen: "DownwardAPI", Fuente: "limits.cpu"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("env = %s\nwant %s", mostrar(got), mostrar(want))
	}
}

func TestLaFotoNoFiltraValoresDeSecret(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	res, err := http.Get(kviz.srv.URL + "/api/foto")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	cuerpo, _ := io.ReadAll(res.Body)

	if filtra(string(cuerpo), contrasenaFalsa) {
		t.Errorf("la foto trae el valor del Secret: %s", cuerpo)
	}
	if !strings.Contains(string(cuerpo), "POSTGRES_PASSWORD") {
		t.Errorf("la foto no trae la clave del Secret: %s", cuerpo)
	}
}

func TestElStreamNoFiltraValoresDeSecret(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)
	stream := kviz.abrirStreamCrudo(t)

	if m := stream.siguiente(t); filtra(m, contrasenaFalsa) {
		t.Errorf("la foto del stream trae el valor del Secret: %s", m)
	}

	// cambiar el valor y sumar una clave: el cambio viaja con las claves, nunca con los valores
	s := secret("default", "postgres-secret", map[string]string{"POSTGRES_PASSWORD": "otro-valor-falso", "POSTGRES_USER": "usuario-falso"})
	if _, err := kviz.cliente.CoreV1().Secrets("default").Update(context.Background(), s, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	for {
		m := stream.siguiente(t)
		for _, v := range []string{contrasenaFalsa, "otro-valor-falso", "usuario-falso"} {
			if filtra(m, v) {
				t.Fatalf("el stream trae un valor del Secret: %s", m)
			}
		}
		if strings.Contains(m, "POSTGRES_USER") {
			return // llegó el cambio del Secret (y del env de postgres)
		}
	}
}

// filtra dice si el texto trae el valor, tal cual o en base64 (como lo guarda Kubernetes).
func filtra(texto, valor string) bool {
	return strings.Contains(texto, valor) || strings.Contains(texto, base64.StdEncoding.EncodeToString([]byte(valor)))
}

// streamCrudo lee los mensajes del stream sin decodificarlos, para buscar texto en el JSON.
type streamCrudo struct{ mensajes <-chan string }

func (k *kvizDePrueba) abrirStreamCrudo(t *testing.T) *streamCrudo {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, k.srv.URL+"/api/stream", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	mensajes := make(chan string, 100)
	go func() {
		defer res.Body.Close()
		defer close(mensajes)
		lector := bufio.NewScanner(res.Body)
		lector.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for lector.Scan() {
			if datos, ok := strings.CutPrefix(lector.Text(), "data: "); ok {
				mensajes <- datos
			}
		}
	}()
	return &streamCrudo{mensajes: mensajes}
}

func (s *streamCrudo) siguiente(t *testing.T) string {
	t.Helper()
	select {
	case m, ok := <-s.mensajes:
		if !ok {
			t.Fatal("el stream se cerró")
		}
		return m
	case <-time.After(esperaStream):
		t.Fatal("no llegó ningún mensaje por el stream")
	}
	return ""
}

func TestUnConfigMapModificadoCambiaEnVivoElEnvDeSusPods(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nginxDeLaGuia()...)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	cm := configMap("default", "nginx-env", map[string]string{"NGINX_PORT": "8080"})
	if _, err := kviz.cliente.CoreV1().ConfigMaps("default").Update(context.Background(), cm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	// el pod llega en el mismo mensaje que el ConfigMap
	var p podConEnv
	if c := stream.esperarCambio(t, "pods", "default/nginx"); json.Unmarshal(c.Valor, &p) != nil || *p.Containers[0].Env[0].Valor != "8080" {
		t.Fatalf("cambio = %s, want nginx con NGINX_PORT=8080", mostrar(c))
	}
}

func TestUnPodSinVolumenesNiEnvTraeListasVacias(t *testing.T) {
	p := pod("default", "shell", "lab-worker", "10.244.1.8", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{Name: "shell", Image: "busybox:latest"}}
	kviz := nuevoKviz(t, "kind-lab", p)

	crudo := kviz.fotoCruda(t)
	var pods []struct {
		Volumenes  json.RawMessage `json:"volumenes"`
		Containers []struct {
			Env      json.RawMessage `json:"env"`
			Montajes json.RawMessage `json:"montajes"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(crudo["pods"], &pods); err != nil {
		t.Fatal(err)
	}
	c := pods[0].Containers[0]
	if string(pods[0].Volumenes) != "[]" || string(c.Env) != "[]" || string(c.Montajes) != "[]" {
		t.Errorf("pod = %s, want volumenes, env y montajes como listas vacías", crudo["pods"])
	}
}

// ---------- fixtures de la guía ----------

const plantillaNginx = `server {
  listen      ${NGINX_PORT};
  server_name localhost;
}
`

// nginxDeLaGuia es 5a-nginx.yaml: dos ConfigMaps, uno como env (envFrom) y otro como volumen.
func nginxDeLaGuia() []runtime.Object {
	p := pod("default", "nginx", "lab-worker", "10.244.1.7", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{
		Name:         "nginx",
		Image:        "nginx:latest",
		Ports:        []corev1.ContainerPort{{ContainerPort: 80}},
		EnvFrom:      []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "nginx-env"}}}},
		VolumeMounts: []corev1.VolumeMount{{Name: "templates", MountPath: "/etc/nginx/templates", ReadOnly: true}},
	}}
	p.Spec.Volumes = []corev1.Volume{{
		Name:         "templates",
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "nginx-template"}}},
	}}
	return []runtime.Object{
		configMap("default", "nginx-template", map[string]string{"default.conf.template": plantillaNginx}),
		configMap("default", "nginx-env", map[string]string{"NGINX_PORT": "80"}),
		p,
	}
}

// backendDeLaGuia es 6b-deployment.yaml: el Deployment backend con su ReplicaSet y dos pods,
// cada uno con NAME tomado de su metadata.name.
func backendDeLaGuia() []runtime.Object {
	d := deployment("default", "backend")
	rs := replicaSet(d, "backend-79f9ffc666", 2, 1)
	objetos := []runtime.Object{d, rs}
	for _, nombre := range []string{"backend-79f9ffc666-k2x8p", "backend-79f9ffc666-7hw4q"} {
		p := podDe(rs, nombre, "lab-worker")
		p.Spec.Containers = []corev1.Container{{
			Name:  "application",
			Image: "lab-backend:latest",
			Env: []corev1.EnvVar{
				{Name: "SERVER_PORT", Value: "8081"},
				{Name: "NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
				{Name: "VERSION", Value: "v1"},
			},
			Ports: []corev1.ContainerPort{{ContainerPort: 8081}},
		}}
		objetos = append(objetos, p)
	}
	return objetos
}

// contrasenaFalsa reemplaza a la contraseña de 5b-postgres.yaml, que no se copia a los tests.
const contrasenaFalsa = "valor-falso-de-fixture"

// postgresDeLaGuia es 5b-postgres.yaml, con un valor falso en el Secret.
func postgresDeLaGuia() []runtime.Object {
	p := pod("default", "postgres", "lab-worker2", "10.244.2.6", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{
		Name:    "main",
		Image:   "postgres:latest",
		Ports:   []corev1.ContainerPort{{ContainerPort: 5432}},
		EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-secret"}}}},
	}}
	return []runtime.Object{secret("default", "postgres-secret", map[string]string{"POSTGRES_PASSWORD": contrasenaFalsa}), p}
}

// secret arma un Secret Opaque como lo guarda la API: los valores en data (stringData es solo
// para escribir).
func secret(namespace, nombre string, datos map[string]string) *corev1.Secret {
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: nombre}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{}}
	for k, v := range datos {
		s.Data[k] = []byte(v)
	}
	return s
}

func configMap(namespace, nombre string, datos map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: nombre}, Data: datos}
}
