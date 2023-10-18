import { AxiosError } from 'axios';

import * as AlertUtils from '../utils/AlertUtils';
import * as API from '../services/Api';
import { KialiDispatch } from '../types/Redux';
import { TracingActions } from './TracingActions';
import { setTraceId as setURLTraceId } from 'utils/SearchParamUtils';
import { transformTraceData } from 'utils/tracing/TraceTransform';

export const TracingThunkActions = {
  setTraceId: (cluster?: string, traceId?: string, tab?: boolean) => {
    setURLTraceId(traceId);
    return (dispatch: KialiDispatch) => {
      if (traceId) {
        API.getTrace(traceId)
          .then(response => {
            if (response.data.data) {
              const trace = transformTraceData(response.data.data, cluster);
              if (trace) {
                trace.loaded = true;
                dispatch(TracingActions.setTrace(trace));
                if (tab) {
                  dispatch(TracingActions.tabTrace(trace));
                }
              }
            }
          })
          .catch(error => {
            if ((error as AxiosError).response?.status === 404) {
              setURLTraceId(undefined);
            }
            dispatch(TracingActions.setTrace(undefined));
            dispatch(TracingActions.tabTrace(undefined));
            AlertUtils.addMessage({
              ...AlertUtils.extractAxiosError('Could not fetch trace', error),
              showNotification: false
            });
          });
      } else {
        dispatch(TracingActions.setTrace(undefined));
        dispatch(TracingActions.tabTrace(undefined));
      }
    };
  }
};
