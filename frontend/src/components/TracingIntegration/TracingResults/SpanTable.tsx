import * as React from 'react';
import { connect } from 'react-redux';
import { KialiDispatch } from 'types/Redux';
import {
  Button,
  ButtonVariant,
  EmptyState,
  EmptyStateBody,
  EmptyStateVariant,
  EmptyStateHeader,
  Icon
} from '@patternfly/react-core';
import {
  TableVariant,
  SortByDirection,
  IRow,
  IRowData,
  IAction,
  Table,
  Thead,
  Tbody,
  Tr,
  Td,
  ThProps,
  Th,
  IRowCell,
  ActionsColumn
} from '@patternfly/react-table';
import { compareNullable } from 'components/FilterList/FilterHelper';
import { MetricsStats } from 'types/Metrics';
import { KialiAppState } from 'store/Store';
import { MetricsStatsQuery } from 'types/MetricsOptions';
import { MetricsStatsThunkActions } from 'actions/MetricsStatsThunkActions';
import { EnvoySpanInfo, OpenTracingHTTPInfo, OpenTracingTCPInfo, RichSpanData } from 'types/TracingInfo';
import { sameSpans } from 'utils/tracing/TracingHelper';
import { buildQueriesFromSpans } from 'utils/tracing/TraceStats';
import { getParamsSeparator, getSpanId } from '../../../utils/SearchParamUtils';
import { kialiStyle } from 'styles/StyleUtils';
import { ExclamationCircleIcon } from '@patternfly/react-icons';
import { formatDuration, isErrorTag } from 'utils/tracing/TracingHelper';
import { Link } from 'react-router-dom';
import { PFColors } from 'components/Pf/PfColors';
import { responseFlags } from 'utils/ResponseFlags';
import { renderMetricsComparison } from './StatsComparison';
import { history } from 'app/History';
import { AngleDownIcon, AngleRightIcon, ExternalLinkAltIcon } from '@patternfly/react-icons';
import { isParentKiosk, kioskContextMenuAction } from '../../Kiosk/KioskActions';
import { TEMPO } from '../../../types/Tracing';

type ReduxProps = {
  kiosk: string;
  loadMetricsStats: (queries: MetricsStatsQuery[], isCompact: boolean) => void;
  metricsStats: Map<string, MetricsStats>;
  provider?: string;
};

type Props = ReduxProps & {
  cluster?: string;
  externalURL?: string;
  items: RichSpanData[];
  namespace: string;
  traceID: string;
};

interface State {
  expandedSpans: Map<string, boolean>;
  openKebab?: string;
  sortDirection: SortByDirection;
  sortIndex: number;
}

type SortableTh<T> = ThProps & {
  compare?: (a: T, b: T) => number;
  sortable: boolean;
};

const dangerErrorStyle = kialiStyle({
  borderLeft: '3px solid var(--pf-v5-global--danger-color--100)'
});

const selectedErrorStyle = kialiStyle({
  borderRight: '3px solid var(--pf-v5-global--info-color--100)',
  borderLeft: '3px solid var(--pf-v5-global--danger-color--100)'
});

const selectedStyle = kialiStyle({
  borderRight: '3px solid var(--pf-v5-global--info-color--100)'
});

const rowKebabStyle = kialiStyle({
  paddingLeft: 0,
  textAlign: 'left',
  whiteSpace: 'nowrap'
});

const linkStyle = kialiStyle({
  fontSize: '14px'
});

const getClassName = (isError: boolean, isSpan: boolean): string | undefined => {
  return isSpan ? (isError ? selectedErrorStyle : selectedStyle) : isError ? dangerErrorStyle : undefined;
};

const columns: SortableTh<RichSpanData>[] = [
  {
    title: 'Timeline',
    sortable: true,
    compare: (a, b) => a.startTime - b.startTime
  },
  {
    title: 'App / Workload',
    sortable: true,
    compare: (a, b) => compareNullable(a.workload, b.workload, (a2, b2) => a2.localeCompare(b2))
  },
  {
    title: 'Summary',
    sortable: false
  },
  {
    title: 'Statistics',
    sortable: true,
    compare: (a, b) => a.duration - b.duration
  }
];

class SpanTableComponent extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    const isSpan = getSpanId();
    const mapExpandedSpans = new Map();
    isSpan && mapExpandedSpans.set(isSpan, true);
    this.state = {
      expandedSpans: mapExpandedSpans,
      sortIndex: 0,
      sortDirection: SortByDirection.asc
    };
  }

  componentDidMount() {
    this.fetchComparisonMetrics(this.props.items);
  }

  componentDidUpdate(prevProps: Readonly<Props>) {
    if (!sameSpans(prevProps.items, this.props.items)) {
      this.fetchComparisonMetrics(this.props.items);
    }
  }

  render() {
    const rows = this.rows();

    return (
      <Table variant={TableVariant.compact} aria-label={'list_spans'}>
        <Thead>
          <Tr>
            {columns.map((column, index) => (
              <Th sort={this.getSortParams(index)}>{column.title}</Th>
            ))}
          </Tr>
        </Thead>

        <Tbody>
          {this.props.items.length > 0 ? (
            rows.map((row, index) => (
              <Tr key={`row_${index}`} className={row.className}>
                {(row.cells as IRowCell[])?.map((cell, index) => (
                  <Td dataLabel={columns[index].title}>{cell}</Td>
                ))}
                <Td isActionCell>
                  <ActionsColumn items={this.actionResolver(row)} />
                </Td>
              </Tr>
            ))
          ) : (
            <Tr>
              <Td colSpan={columns.length}>
                <EmptyState variant={EmptyStateVariant.full}>
                  <EmptyStateHeader titleText="No spans found" headingLevel="h5" />
                  <EmptyStateBody>No spans match the current filters</EmptyStateBody>
                </EmptyState>
              </Td>
            </Tr>
          )}
        </Tbody>
      </Table>
    );
  }

  private getSortParams = (columnIndex: number): ThProps['sort'] => {
    return columns[columnIndex].sortable
      ? {
          sortBy: { index: this.state.sortIndex, direction: this.state.sortDirection },
          onSort: (_event, index, sortDirection) => this.setState({ sortIndex: index, sortDirection: sortDirection }),
          columnIndex
        }
      : undefined;
  };

  private fetchComparisonMetrics(items: RichSpanData[]) {
    const queries = buildQueriesFromSpans(items, false);
    this.props.loadMetricsStats(queries, false);
  }

  private rows = (): IRow[] => {
    const compare = columns[this.state.sortIndex].compare;
    const sorted = compare
      ? this.props.items.sort(this.state.sortDirection === SortByDirection.asc ? compare : (a, b) => compare(b, a))
      : this.props.items;

    return sorted.map(item => this.buildRow(item));
  };

  private buildRow = (item: RichSpanData): IRow => {
    const isExpanded = this.isExpanded(item.spanID);
    const isSpan = item.spanID === getSpanId();

    return {
      cells: [
        <>
          <Button
            key={`${item.spanID}-duration`}
            style={{ padding: '6px 4px 6px 0' }}
            variant={ButtonVariant.link}
            onClick={() => this.toggleExpanded(item.spanID)}
          >
            {isExpanded ? <AngleDownIcon /> : <AngleRightIcon />}
          </Button>
          {formatDuration(item.relativeStartTime)}
        </>,
        this.originCell(item),
        this.summaryCell(item),
        this.StatsCell(item)
      ] as React.ReactNode[],
      className: getClassName(item.tags.some(isErrorTag), isSpan),
      item: item
    };
  };

  private actionResolver = (rowData: IRowData): IAction[] => {
    const item = rowData.item;
    const parentKiosk = isParentKiosk(this.props.kiosk);
    const appActions: IAction[] = [
      {
        isDisabled: true,
        title: (
          <h1
            className={`pf-c-dropdown__group-title ${rowKebabStyle}`}
            aria-hidden="true"
          >{`Application (${item.app})`}</h1>
        )
      },
      {
        title: 'Inbound Metrics',
        onClick: () => {
          const href = item.linkToApp + getParamsSeparator(rowData.item.linkToApp) + 'tab=in_metrics';
          if (parentKiosk) {
            kioskContextMenuAction(href);
          } else {
            history.push(href);
          }
        }
      },
      {
        title: 'Outbound Metrics',
        onClick: () => {
          const href = item.linkToApp + getParamsSeparator(rowData.item.linkToApp) + 'tab=out_metrics';
          if (parentKiosk) {
            kioskContextMenuAction(href);
          } else {
            history.push(href);
          }
        }
      }
    ];

    let workloadActions: IAction[] = [];
    if (item.linkToWorkload) {
      workloadActions = [
        {
          isDisabled: true,
          title: (
            <h1
              className={`pf-c-dropdown__group-title ${rowKebabStyle}`}
              aria-hidden="true"
            >{`Workload (${item.workload})`}</h1>
          )
        },
        {
          title: 'Logs',
          onClick: () => {
            const href = item.linkToWorkload + '?tab=logs';
            if (parentKiosk) {
              kioskContextMenuAction(href);
            } else {
              history.push(href);
            }
          }
        },
        {
          title: 'Inbound Metrics',
          onClick: () => {
            const href = item.linkToWorkload + getParamsSeparator(rowData.item.linkToWorkload) + 'tab=in_metrics';
            if (parentKiosk) {
              kioskContextMenuAction(href);
            } else {
              history.push(href);
            }
          }
        },
        {
          title: 'Outbound Metrics',
          onClick: () => {
            const href = item.linkToWorkload + getParamsSeparator(rowData.item.linkToWorkload) + 'tab=out_metrics';
            if (parentKiosk) {
              kioskContextMenuAction(href);
            } else {
              history.push(href);
            }
          }
        }
      ];
    }

    let tracingActions: IAction[] = [];
    if (this.props.externalURL) {
      const traceURL = this.props.externalURL?.replace('TRACEID', this.props.traceID);
      const spanLink = this.props.provider === TEMPO ? traceURL : `${traceURL}?uiFind=${item.spanID}`;
      tracingActions = [
        {
          isDisabled: true,
          title: <h1 className={`pf-c-dropdown__group-title ${rowKebabStyle}`} aria-hidden="true">{`Tracing`}</h1>
        },
        {
          title: (
            <span className={linkStyle}>
              More span details <ExternalLinkAltIcon />
            </span>
          ),
          onClick: () => window.open(spanLink, '_blank')
        }
      ];
    }

    // Parent Kiosk won't have links to the app details
    // as most of the kubernetes consoles don't have an unified "app" entity
    return parentKiosk
      ? [...workloadActions, ...tracingActions]
      : [...appActions, ...workloadActions, ...tracingActions];
  };

  private isExpanded = (spanID: string): boolean => {
    return this.state.expandedSpans.get(spanID) || false;
  };

  private toggleExpanded = (spanID: string): void => {
    const isExpanded = this.isExpanded(spanID);
    this.state.expandedSpans.set(spanID, !isExpanded);
    this.setState({ expandedSpans: this.state.expandedSpans });
  };

  private originCell = (item: RichSpanData): React.ReactNode => {
    const parentKiosk = isParentKiosk(this.props.kiosk);
    const key = `${item.spanID}-origin`;
    return (
      <>
        <strong key={`${key}-app`}>Application: </strong>
        {(item.linkToApp &&
          (parentKiosk ? (
            <Link
              key={`${key}-link-app`}
              to={''}
              onClick={() => {
                if (item.linkToApp) {
                  kioskContextMenuAction(item.linkToApp);
                }
              }}
            >
              {item.app}
            </Link>
          ) : (
            <Link key={`${key}-link-app`} to={item.linkToApp}>
              {item.app}
            </Link>
          ))) ||
          item.app}
        <br key={`${key}-br`} />
        <strong key={`${key}-wl`}>Workload: </strong>
        {(item.linkToWorkload &&
          (parentKiosk ? (
            <Link
              key={`${key}-link-wl`}
              to={''}
              onClick={() => {
                if (item.linkToWorkload) {
                  kioskContextMenuAction(item.linkToWorkload);
                }
              }}
            >
              {item.workload}
            </Link>
          ) : (
            <Link key={`${key}-link-wl`} to={item.linkToWorkload}>
              {item.workload}
            </Link>
          ))) ||
          'unknown'}
        {this.isExpanded(item.spanID) && (
          <div key={`${key}-expanded-br-1`}>
            <strong key={`${key}-expanded-pod`}>Pod: </strong>
            {item.pod || 'unknown'}
          </div>
        )}
      </>
    );
  };

  private summaryCell = (item: RichSpanData): React.ReactNode => {
    const flag = (item.info as EnvoySpanInfo).responseFlags;
    const key = `${item.spanID}-summary`;
    return (
      <>
        {item.info.hasError && (
          <div key={`${key}-err`}>
            <Icon key={`${key}-err-ic`} color={PFColors.Danger}>
              <ExclamationCircleIcon />
            </Icon>{' '}
            <strong key={`${key}-err-msg`}>This span reported an error</strong>
          </div>
        )}
        <div key={`${key}-op`}>
          <strong key={`${key}-op-title`}>Operation: </strong>
          {flag ? (
            <span key={`${key}-op-name`}>
              {item.operationName} ({flag}{' '}
              <Icon key={`${key}-dan-ic`} color={PFColors.Danger}>
                <ExclamationCircleIcon />
              </Icon>
              )
            </span>
          ) : (
            <span key={`${key}-op-name`}>{item.operationName}</span>
          )}
        </div>
        <div key={`${key}-comp`}>
          <strong key={`${key}-comp=-title`}>Component: </strong>
          {item.component}
        </div>
        {this.isExpanded(item.spanID) &&
          ((item.type === 'envoy' && this.renderEnvoySummary(item)) ||
            (item.type === 'http' && this.renderHTTPSummary(item)) ||
            (item.type === 'tcp' && this.renderTCPSummary(item)))}
      </>
    );
  };

  private renderEnvoySummary = (item: RichSpanData) => {
    const parentKiosk = isParentKiosk(this.props.kiosk);
    const info = item.info as EnvoySpanInfo;
    let rqLabel = 'Request';
    let peerLink: JSX.Element | undefined = undefined;
    const key = `${item.spanID}-summary-envoy`;
    if (info.direction === 'inbound') {
      rqLabel = 'Received request';
      if (info.peer) {
        peerLink = (
          <>
            {' from '}
            {parentKiosk ? (
              <Link
                to={''}
                onClick={() => {
                  if (info.peer) {
                    kioskContextMenuAction('/namespaces/' + info.peer.namespace + '/workloads/' + info.peer.name);
                  }
                }}
              >
                {info.peer.name}
              </Link>
            ) : (
              <Link to={'/namespaces/' + info.peer.namespace + '/workloads/' + info.peer.name}>{info.peer.name}</Link>
            )}
          </>
        );
      }
    } else if (info.direction === 'outbound') {
      rqLabel = 'Sent request';
      if (info.peer) {
        peerLink = (
          <React.Fragment key={`${key}-out`}>
            <span key={`${key}-out-to`}>{' to '}</span>
            {parentKiosk ? (
              <Link
                key={`${key}-out-link`}
                to={''}
                onClick={() => {
                  if (info.peer) {
                    kioskContextMenuAction('/namespaces/' + info.peer.namespace + '/services/' + info.peer.name);
                  }
                }}
              >
                {info.peer.name}
              </Link>
            ) : (
              <Link key={`${key}-out-link`} to={'/namespaces/' + info.peer.namespace + '/services/' + info.peer.name}>
                {info.peer.name}
              </Link>
            )}
          </React.Fragment>
        );
      }
    }
    const rsDetails: string[] = [];
    if (info.statusCode) {
      rsDetails.push(String(info.statusCode));
    }
    let flagInfo: string | undefined = undefined;
    if (info.responseFlags) {
      rsDetails.push(info.responseFlags);
      flagInfo = responseFlags[info.responseFlags]?.help || 'Unknown flag';
    }

    return (
      <React.Fragment key={`${key}`}>
        <div key={`${key}-req`}>
          <strong key={`${key}-req-title`}>
            {rqLabel}
            {peerLink}:{' '}
          </strong>
          <span key={`${key}-req-val`}>
            {info.method} {info.url}
          </span>
        </div>
        <div key={`${key}-status`}>
          <strong key={`${key}-status-title`}>Response status: </strong>
          <span key={`${key}-status-val`}>{rsDetails.join(', ')}</span>
        </div>
        <span key={`${key}-flag`}>{flagInfo}</span>
      </React.Fragment>
    );
  };

  private renderHTTPSummary = (item: RichSpanData) => {
    const info = item.info as OpenTracingHTTPInfo;
    const rqLabel =
      info.direction === 'inbound' ? 'Received request' : info.direction === 'outbound' ? 'Sent request' : 'Request';
    const key = `${item.spanID}-summary-http`;
    return (
      <React.Fragment key={key}>
        <div key={`${key}-req`}>
          <strong key={`${key}-req-title`}>{rqLabel}: </strong>
          <span key={`${key}-req-val`}>
            {info.method} {info.url}
          </span>
        </div>
        {info.statusCode && (
          <div key={`${key}-code`}>
            <strong key={`${key}-code-title`}>Response status: </strong>
            <span key={`${key}-code-val`}>{info.statusCode}</span>
          </div>
        )}
      </React.Fragment>
    );
  };

  private renderTCPSummary = (item: RichSpanData) => {
    const info = item.info as OpenTracingTCPInfo;
    const key = `${item.spanID}-summary-tcp`;
    return (
      <React.Fragment key={key}>
        {info.topic && (
          <div key={`${key}-topic`}>
            <strong key={`${key}-topic-title`}>Topic: </strong>
            <span key={`${key}-topic-val`}>{info.topic}</span>
          </div>
        )}
      </React.Fragment>
    );
  };

  private StatsCell = (item: RichSpanData): React.ReactNode => {
    const key = `${item.spanID}-stats`;
    return (
      <div key={key}>
        <div key={`${key}-dur-div`}>
          <strong key={`${key}-dur-title`}>Duration: </strong>
          {formatDuration(item.duration)}
        </div>
        {item.type === 'envoy' &&
          renderMetricsComparison(item, !this.isExpanded(item.spanID), this.props.metricsStats, () =>
            this.fetchComparisonMetrics([item])
          )}
      </div>
    );
  };
}

const mapStateToProps = (state: KialiAppState) => ({
  kiosk: state.globalState.kiosk,
  metricsStats: state.metricsStats.data,
  provider: state.tracingState.info?.provider
});

const mapDispatchToProps = (dispatch: KialiDispatch) => ({
  loadMetricsStats: (queries: MetricsStatsQuery[], isCompact: boolean) =>
    dispatch(MetricsStatsThunkActions.load(queries, isCompact))
});

export const SpanTable = connect(mapStateToProps, mapDispatchToProps)(SpanTableComponent);
