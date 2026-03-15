{{- define "civika.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "civika.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "civika.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "civika.labels" -}}
helm.sh/chart: {{ include "civika.chart" . }}
app.kubernetes.io/name: {{ include "civika.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "civika.selectorLabels" -}}
app.kubernetes.io/name: {{ include "civika.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "civika.backendName" -}}
{{- printf "%s-backend" (include "civika.fullname" .) -}}
{{- end -}}

{{- define "civika.frontendName" -}}
{{- printf "%s-frontend" (include "civika.fullname" .) -}}
{{- end -}}

{{- define "civika.backendServiceName" -}}
{{- include "civika.backendName" . -}}
{{- end -}}

{{- define "civika.frontendServiceName" -}}
{{- include "civika.frontendName" . -}}
{{- end -}}

{{- define "civika.backendSecretName" -}}
{{- if .Values.backend.secrets.existingSecret -}}
{{- .Values.backend.secrets.existingSecret -}}
{{- else -}}
{{- printf "%s-backend-secrets" (include "civika.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresCredentialsSecretName" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
  {{- if .Values.postgresql.managed.existingCredentialsSecret -}}
    {{- .Values.postgresql.managed.existingCredentialsSecret -}}
  {{- else -}}
    {{- printf "%s-postgres-app" (include "civika.fullname" .) -}}
  {{- end -}}
{{- else -}}
  {{- if .Values.postgresql.external.existingCredentialsSecret -}}
    {{- .Values.postgresql.external.existingCredentialsSecret -}}
  {{- else -}}
    {{- printf "%s-postgres-external" (include "civika.fullname" .) -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresClusterName" -}}
{{- printf "%s-postgres" (include "civika.fullname" .) -}}
{{- end -}}

{{- define "civika.postgresRwHost" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
{{- printf "%s-rw" (include "civika.postgresClusterName" .) -}}
{{- else -}}
{{- .Values.postgresql.external.rwHost -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresRoHost" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
{{- printf "%s-ro" (include "civika.postgresClusterName" .) -}}
{{- else -}}
{{- .Values.postgresql.external.roHost -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresPort" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
{{- .Values.postgresql.port -}}
{{- else -}}
{{- .Values.postgresql.external.port -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresUser" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
{{- .Values.postgresql.user -}}
{{- else -}}
{{- .Values.postgresql.external.user -}}
{{- end -}}
{{- end -}}

{{- define "civika.postgresDatabase" -}}
{{- if eq .Values.postgresql.mode "managed" -}}
{{- .Values.postgresql.database -}}
{{- else -}}
{{- .Values.postgresql.external.database -}}
{{- end -}}
{{- end -}}

{{- define "civika.ragDataPvcName" -}}
{{- if .Values.ragChunker.dataVolume.existingClaim -}}
{{- .Values.ragChunker.dataVolume.existingClaim -}}
{{- else -}}
{{- printf "%s-rag-data" (include "civika.fullname" .) -}}
{{- end -}}
{{- end -}}
