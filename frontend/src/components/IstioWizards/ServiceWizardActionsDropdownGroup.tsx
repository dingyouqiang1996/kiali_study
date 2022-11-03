import * as React from "react";
import { DropdownGroup, DropdownItem, DropdownSeparator, Tooltip, TooltipPosition } from "@patternfly/react-core";
import {serverConfig} from "config";
import {
  DestinationRule,
  getWizardUpdateLabel,
  K8sHTTPRoute,
  VirtualService
} from "types/IstioObjects";
import { canDelete, ResourcePermissions } from "types/Permissions";
import {
  SERVICE_WIZARD_ACTIONS,
  WIZARD_K8S_REQUEST_ROUTING,
  WIZARD_TITLES,
  WizardAction,
  WizardMode
} from "./WizardActions";
import { hasServiceDetailsTrafficRouting } from "../../types/ServiceInfo";

export const DELETE_TRAFFIC_ROUTING = 'delete_traffic_routing';

type Props = {
  isDisabled?: boolean;
  destinationRules: DestinationRule[];
  virtualServices: VirtualService[];
  k8sHTTPRoutes: K8sHTTPRoute[];
  istioPermissions: ResourcePermissions;
  onAction?: (key: WizardAction, mode: WizardMode) => void;
  onDelete?: (key: string) => void;
}

const ServiceWizardActionsDropdownGroup: React.FunctionComponent<Props> = props => {
  const updateLabel = getWizardUpdateLabel(props.virtualServices, props.k8sHTTPRoutes);

  function hasTrafficRouting() {
    return hasServiceDetailsTrafficRouting(props.virtualServices, props.destinationRules, props.k8sHTTPRoutes);
  }

  function handleActionClick(eventKey: string) {
    if (props.onAction) {
      props.onAction(eventKey as WizardAction, updateLabel.length === 0 ? 'create' : 'update');
    }
  }

  function getDropdownItemTooltipMessage(): string {
    if (serverConfig.deployment.viewOnlyMode) {
      return 'User does not have permission';
    } else if (hasTrafficRouting()) {
      return 'Traffic routing already exists for this service';
    } else {
      return "Traffic routing doesn't exists for this service";
    }
  }

  const actionItems = SERVICE_WIZARD_ACTIONS.map(eventKey => {
    const enabled = (eventKey === WIZARD_K8S_REQUEST_ROUTING ? serverConfig.gatewayAPIEnabled && !props.isDisabled : !props.isDisabled);
    const enabledItem = enabled && (!hasTrafficRouting() || (hasTrafficRouting() && updateLabel === eventKey));
    const wizardItem = (
      <DropdownItem key={eventKey} component="button" isDisabled={!enabledItem} onClick={() => handleActionClick(eventKey)} data-test={eventKey}>
        {WIZARD_TITLES[eventKey]}
      </DropdownItem>
    );

    // An Item is rendered under two conditions:
    // a) No traffic -> Wizard can create new one
    // b) Existing traffic generated by the traffic -> Wizard can update that scenario
    // Otherwise, the item should be disabled
    if (!enabledItem) {
      return (
        <Tooltip key={'tooltip_' + eventKey} position={TooltipPosition.left} content={<>{getDropdownItemTooltipMessage()}</>}>
          <div style={{ display: 'inline-block', cursor: 'not-allowed' }}>{wizardItem}</div>
        </Tooltip>
      )
    } else {
      return wizardItem;
    }
  });

  actionItems.push(<DropdownSeparator key="actions_separator" />);

  const deleteDisabled = !canDelete(props.istioPermissions) || !hasTrafficRouting() || props.isDisabled;
  let deleteDropdownItem = (
    <DropdownItem
      key={DELETE_TRAFFIC_ROUTING}
      component="button"
      onClick={() => {if (props.onDelete) { props.onDelete(DELETE_TRAFFIC_ROUTING); }}}
      isDisabled={deleteDisabled}
      data-test={DELETE_TRAFFIC_ROUTING}
    >
      Delete Traffic Routing
    </DropdownItem>
  );

  if (deleteDisabled) {
    deleteDropdownItem = (
      <Tooltip key={'tooltip_' + DELETE_TRAFFIC_ROUTING} position={TooltipPosition.left} content={<>{getDropdownItemTooltipMessage()}</>}>
        <div style={{ display: 'inline-block', cursor: 'not-allowed' }}>{deleteDropdownItem}</div>
      </Tooltip>
    );
  }

  actionItems.push(deleteDropdownItem);
  const label = updateLabel === '' ? 'Create' : 'Update';
  return (
    <DropdownGroup
      key={`group_${label}`}
      label={label}
      className="kiali-group-menu"
      children={actionItems}
    />
  );
}

export default ServiceWizardActionsDropdownGroup;
