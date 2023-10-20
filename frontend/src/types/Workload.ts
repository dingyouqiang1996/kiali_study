import { Namespace } from './Namespace';
import { WorkloadHealth, WorkloadHealthResponse } from './Health';
import { ObjectReference, Pod, Service, Validations } from './IstioObjects';

export interface WorkloadId {
  namespace: string;
  workload: string;
}

export interface Workload {
  additionalDetails: AdditionalItem[];
  annotations: { [key: string]: string };
  appLabel: boolean;
  availableReplicas: Number;
  name: string;
  cluster?: string;
  createdAt: string;
  health?: WorkloadHealthResponse;
  istioAmbient: boolean;
  istioInjectionAnnotation?: boolean;
  istioSidecar: boolean;
  labels: { [key: string]: string };
  pods: Pod[];
  replicas: Number;
  resourceVersion: string;
  runtimes: Runtime[];
  services: Service[];
  type: string;
  validations?: Validations;
  versionLabel: boolean;
  waypointWorkloads: Workload[];
}

export const emptyWorkload: Workload = {
  additionalDetails: [],
  annotations: {},
  appLabel: false,
  availableReplicas: 0,
  createdAt: '',
  istioAmbient: false,
  istioSidecar: true, // true until proven otherwise
  labels: {},
  name: '',
  pods: [],
  replicas: 0,
  resourceVersion: '',
  runtimes: [],
  services: [],
  type: '',
  versionLabel: false,
  waypointWorkloads: []
};

export const WorkloadType = {
  CronJob: 'CronJob',
  DaemonSet: 'DaemonSet',
  Deployment: 'Deployment',
  DeploymentConfig: 'DeploymentConfig',
  Job: 'Job',
  Pod: 'Pod',
  ReplicaSet: 'ReplicaSet',
  ReplicationController: 'ReplicationController',
  StatefulSet: 'StatefulSet'
};

export interface WorkloadOverview {
  additionalDetailSample?: AdditionalItem;
  appLabel: boolean;
  cluster?: string;
  health: WorkloadHealth;
  istioAmbient: boolean;
  istioReferences: ObjectReference[];
  istioSidecar: boolean;
  labels: { [key: string]: string };
  name: string;
  notCoveredAuthPolicy: boolean;
  type: string;
  versionLabel: boolean;
}

export interface WorkloadQuery {
  health: 'true' | 'false';
  rateInterval: string;
  validate: 'true' | 'false';
}

export interface WorkloadUpdateQuery {
  type: string;
  patchType?: string;
}

export interface WorkloadListItem extends WorkloadOverview {
  namespace: string;
}

export interface WorkloadListQuery {
  health: 'true' | 'false';
  istioResources: 'true' | 'false';
  rateInterval: string;
}

export interface WorkloadNamespaceResponse {
  namespace: Namespace;
  validations: Validations;
  workloads: WorkloadOverview[];
}

export interface Runtime {
  dashboardRefs: DashboardRef[];
  name: string;
}

export interface DashboardRef {
  template: string;
  title: string;
}

export interface AdditionalItem {
  icon?: string;
  title: string;
  value: string;
}
