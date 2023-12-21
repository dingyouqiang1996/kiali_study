import { FILTER_ACTION_APPEND, FILTER_ACTION_UPDATE, AllFilterTypes, FilterValue, RunnableFilter } from 'types/Filters';
import { RichSpanData } from 'types/TracingInfo';

const byWorkload = (workloads: FilterValue[]): RunnableFilter<RichSpanData> => {
  return {
    category: 'Workload',
    placeholder: $t('Filter.Workload', 'Filter by Workload'),
    filterType: AllFilterTypes.typeAhead,
    action: FILTER_ACTION_APPEND,
    filterValues: workloads,
    run: (item, filters) => filters.filters.some(f => f.value === item.workload)
  };
};

const byApp = (apps: FilterValue[]): RunnableFilter<RichSpanData> => {
  return {
    category: 'App',
    placeholder: $t('Filter.App', 'Filter by App'),
    filterType: AllFilterTypes.typeAhead,
    action: FILTER_ACTION_APPEND,
    filterValues: apps,
    run: (item, filters) => filters.filters.some(f => f.value === item.app)
  };
};

const byComponent = (components: FilterValue[]): RunnableFilter<RichSpanData> => {
  return {
    category: 'Component',
    placeholder: $t('Filter.SpanComponent', 'Filter by Span Component'),
    filterType: AllFilterTypes.typeAhead,
    action: FILTER_ACTION_APPEND,
    filterValues: components,
    run: (item, filters) => filters.filters.some(f => f.value === item.component)
  };
};

const byOperation = (ops: FilterValue[]): RunnableFilter<RichSpanData> => {
  return {
    category: 'Operation',
    placeholder: $t('Filter.OperationName', 'Filter by Operation Name'),
    filterType: AllFilterTypes.typeAhead,
    action: FILTER_ACTION_APPEND,
    filterValues: ops,
    run: (item, filters) => filters.filters.some(f => f.value === item.operationName)
  };
};

const byError: RunnableFilter<RichSpanData> = {
  category: 'Error',
  placeholder: $t('Filter.Error', 'Filter by Error'),
  filterType: AllFilterTypes.select,
  action: FILTER_ACTION_UPDATE,
  filterValues: [
    { id: 'yes', title: $t('WithErrors', 'With errors') },
    { id: 'no', title: $t('WithoutErrors', 'Without errors') }
  ],
  run: (item, filters) =>
    filters.filters.some(
      f => f.value === (item.info.hasError ? $t('WithErrors', 'With errors') : $t('WithoutErrors', 'Without errors'))
    )
};

export const spanFilters = (spans: RichSpanData[]): RunnableFilter<RichSpanData>[] => {
  const workloads = new Set<string>();
  const apps = new Set<string>();
  const components = new Set<string>();
  const operations = new Set<string>();
  spans.forEach(s => {
    workloads.add(s.workload || 'unknown');
    apps.add(s.app);
    components.add(s.component);
    operations.add(s.operationName);
  });
  return [
    byWorkload(Array.from(workloads).map(w => ({ id: w, title: w }))),
    byApp(Array.from(apps).map(w => ({ id: w, title: w }))),
    byComponent(Array.from(components).map(w => ({ id: w, title: w }))),
    byOperation(Array.from(operations).map(w => ({ id: w, title: w }))),
    byError
  ];
};
