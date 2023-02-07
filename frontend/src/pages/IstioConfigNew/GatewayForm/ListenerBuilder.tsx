import * as React from 'react';
import { isGatewayHostValid } from '../../../utils/IstioConfigUtils';
import { Button, ButtonVariant, FormSelect, FormSelectOption, TextInput } from '@patternfly/react-core';
import { isValid } from '../../../utils/Common';
import { TrashIcon } from '@patternfly/react-icons';
import { ListenerForm } from '../K8sGatewayForm';
import { Td, Tr } from '@patternfly/react-table';
import { addSelectorLabels } from './ListenerList';

type Props = {
  listener: ListenerForm;
  onRemoveListener: (i: number) => void;
  index: number;
  onChange: (listenerForm: ListenerForm, i: number) => void;
};

// Only HTTPRoute is supported in Istio
export const protocols = ['HTTP'];
export const allowedRoutes = ['All', 'Selector', 'Same'];

export const isValidName = (name: string) => {
  return name !== undefined && name.length > 0;
};

export const isValidHostname = (hostname: string) => {
  return hostname !== undefined && hostname.length > 0 && isGatewayHostValid(hostname);
};

export const isValidPort = (port: string) => {
  return port.length > 0 && !isNaN(Number(port)) && Number(port) >= 0 && Number(port) <= 65535;
};

export const isValidSelector = (selector: string) => {
  return selector.length === 0 || typeof addSelectorLabels(selector) !== 'undefined';
};

class ListenerBuilder extends React.Component<Props> {
  isValidHost = (host: string): boolean => {
    return isGatewayHostValid(host);
  };

  onAddHostname = (value: string, _) => {
    const l = this.props.listener;
    l.hostname = value.trim();

    this.props.onChange(l, this.props.index);

    this.setState({
      newHostname: value,
      isHostValid: this.isValidHost(value)
    });
  };

  onAddPort = (value: string, _) => {
    const l = this.props.listener;
    l.port = value.trim();

    this.props.onChange(l, this.props.index);
  };

  onAddName = (value: string, _) => {
    const l = this.props.listener;
    l.name = value.trim();

    this.props.onChange(l, this.props.index);
  };

  onAddProtocol = (value: string, _) => {
    const l = this.props.listener;
    l.protocol = value.trim();

    this.props.onChange(l, this.props.index);
  };

  onAddFrom = (value: string, _) => {
    const l = this.props.listener;
    l.from = value.trim();

    this.props.onChange(l, this.props.index);
  };

  onAddSelectorLabels = (value: string, _) => {
    const l = this.props.listener;
    l.sSelectorLabels = value.trim();

    this.props.onChange(l, this.props.index);
  };

  render() {
    return (
      <Tr>
        <Td>
          <TextInput
            value={this.props.listener.name}
            type="text"
            id="addName"
            aria-describedby="add name"
            onChange={this.onAddName}
            validated={isValid(isValidName(this.props.listener.name))}
          />
        </Td>
        <Td>
          <TextInput
            value={this.props.listener.hostname}
            type="text"
            id="addHostname"
            aria-describedby="add hostname"
            name="addHostname"
            onChange={this.onAddHostname}
            validated={isValid(isValidHostname(this.props.listener.hostname))}
          />
        </Td>
        <Td>
          <TextInput
            value={this.props.listener.port}
            type="text"
            id="addPort"
            placeholder="80"
            aria-describedby="add port"
            name="addPortNumber"
            onChange={this.onAddPort}
            validated={isValid(isValidPort(this.props.listener.port))}
          />
        </Td>
        <Td>
          <FormSelect
            value={this.props.listener.protocol}
            id="addPortProtocol"
            name="addPortProtocol"
            onChange={this.onAddProtocol}
          >
            {protocols.map((option, index) => (
              <FormSelectOption isDisabled={false} key={'p' + index} value={option} label={option} />
            ))}
          </FormSelect>
        </Td>
        <Td>
          <FormSelect value={this.props.listener.from} id="addFrom" name="addFrom" onChange={this.onAddFrom}>
            {allowedRoutes.map((option, index) => (
              <FormSelectOption isDisabled={false} key={'p' + index} value={option} label={option} />
            ))}
          </FormSelect>
        </Td>
        <Td>
          <TextInput
            id="addSelectorLabels"
            name="addSelectorLabels"
            onChange={this.onAddSelectorLabels}
            validated={isValid(isValidSelector(this.props.listener.sSelectorLabels))}
          />
        </Td>
        <Td>
          <Button
            id="deleteBtn"
            variant={ButtonVariant.link}
            icon={<TrashIcon />}
            style={{ padding: 0 }}
            onClick={() => this.props.onRemoveListener(this.props.index)}
          />
        </Td>
      </Tr>
    );
  }
}

export default ListenerBuilder;
