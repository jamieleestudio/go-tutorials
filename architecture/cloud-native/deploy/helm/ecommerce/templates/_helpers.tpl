{{/* 模板助手（对照 Java _helpers.tpl）*/}}

{{- define "ecommerce.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ecommerce.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "ecommerce.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ecommerce.labels" -}}
app.kubernetes.io/name: {{ include "ecommerce.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/* 镜像引用：{{ .Values.image.repository }}/cn-{{ 服务名 }}:{{ tag }}，与 Java 命名对齐 */}}
{{- define "ecommerce.image" -}}
{{- $svc := .service -}}
{{- $root := .root -}}
{{- printf "%s/cn-%s:%s" $root.Values.image.repository $svc.name $root.Values.image.tag -}}
{{- end -}}
