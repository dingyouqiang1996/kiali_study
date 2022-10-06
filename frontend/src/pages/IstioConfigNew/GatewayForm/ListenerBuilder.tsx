import * as React from 'react';
import { Button, ButtonVariant, FormGroup, FormSelect, FormSelectOption } from '@patternfly/react-core';
import { TextInputBase as TextInput } from '@patternfly/react-core/dist/js/components/TextInput/TextInput';
import { cellWidth, ICell, Table, TableBody, TableHeader } from '@patternfly/react-table';
import { style } from 'typestyle';
import { PFColors } from '../../../components/Pf/PfColors';
import { PlusCircleIcon } from '@patternfly/react-icons';
import { isGatewayHostValid } from '../../../utils/IstioConfigUtils';
import { Listener } from '../../../types/IstioObjects';
import { isValid } from 'utils/Common';

type Props = {
  onAddListener: (listener: Listener) => void;
};

type State = {
  isHostValid: boolean;
  newHostname: string;
  newPort: string;
  newName: string;
  newProtocol: string;
  newFrom: string;
};

const warningStyle = style({
  marginLeft: 25,
  color: PFColors.Red100,
  textAlign: 'center'
});

const addListenerStyle = style({
  marginLeft: 0,
  paddingLeft: 0
});

const listenerHeader: ICell[] = [
  {
    title: 'Name',
    transforms: [cellWidth(20) as any],
    props: {}
  },
  {
    title: 'Hostname',
    transforms: [cellWidth(20) as any],
    props: {}
  },
  {
    title: 'Port',
    transforms: [cellWidth(20) as any],
    props: {}
  },
  {
    title: 'Protocol',
    transforms: [cellWidth(20) as any],
    props: {}
  },
  {
    title: 'From Namespaces',
    transforms: [cellWidth(20) as any],
    props: {}
  }
];

// Only HTTPRoute is supported in Istio
const protocols = ['HTTP'];

const allowedRoutes = ['All', 'Selector', 'Same'];


class ListenerBuilder extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      newHostname: '',
      isHostValid: false,
      newPort: '',
      newName: '',
      newProtocol: protocols[0],
      newFrom: allowedRoutes[2],
    };
  }

  canAddListener = (): boolean => {
    const hostValid = this.state.isHostValid;
    const portNumberValid = this.state.newPort.length > 0 && !isNaN(Number(this.state.newPort));
    const portNameValid = this.state.newName.length > 0;
    return hostValid && portNumberValid && portNameValid;
  };

  isValidHost = (host: string): boolean => {
    return isGatewayHostValid(host);
  };

  onAddHostname = (value: string, _) => {
    this.setState({
      newHostname: value,
      isHostValid: this.isValidHost(value)
    });
  };

  onAddPort = (value: string, _) => {
    this.setState({
      newPort: value.trim()
    });
  };

  onAddName = (value: string, _) => {
    this.setState({
      newName: value.trim()
    });
  };

  onAddProtocol = (value: string, _) => {
    this.setState({
      newProtocol: value
    });
  };

  onAddFrom = (value: string, _) => {
    this.setState({
      newFrom: value
    });
  };

  onAddListener = () => {
    const newListener: Listener = {
      hostname: this.state.newHostname,
      port: +this.state.newPort,
      name: this.state.newName,
      protocol: this.state.newProtocol,
      allowedRoutes: {namespaces: {from: this.state.newFrom}}
    };
    this.setState(
      {
        newHostname: '',
        isHostValid: false,
        newPort: '',
        newName: '',
        newProtocol: protocols[0],
        newFrom: allowedRoutes[2],
      },
      () => this.props.onAddListener(newListener)
    );
  };

  portRows() {
    return [
      {
        keys: 'gatewayListenerNew',
        cells: [
          <>
            <TextInput
              value={this.state.newName}
              type="text"
              id="addName"
              aria-describedby="add name"
              name="addName"
              onChange={this.onAddName}
              validated={isValid(this.state.newName.length > 0)}
            />
          </>,
          <>
            <TextInput
              value={this.state.newHostname}
              type="text"
              id="addHostname"
              aria-describedby="add hostname"
              name="addHostname"
              onChange={this.onAddHostname}
              validated={isValid(this.state.newHostname.length > 0)}
            />
          </>,
          <>
            <TextInput
              value={this.state.newPort}
              type="text"
              id="addPort"
              aria-describedby="add port"
              name="addPortNumber"
              onChange={this.onAddPort}
              validated={isValid(this.state.newPort.length > 0 && !isNaN(Number(this.state.newPort)))}
            />
          </>,
          <>
            <FormSelect
              value={this.state.newProtocol}
              id="addPortProtocol"
              name="addPortProtocol"
              onChange={this.onAddProtocol}
            >
              {protocols.map((option, index) => (
                <FormSelectOption isDisabled={false} key={'p' + index} value={option} label={option} />
              ))}
            </FormSelect>
          </>,
          <>
            <FormSelect
              value={this.state.newFrom}
              id="addFrom"
              name="addFrom"
              onChange={this.onAddFrom}
            >
              {allowedRoutes.map((option, index) => (
                <FormSelectOption isDisabled={false} key={'p' + index} value={option} label={option} />
              ))}
            </FormSelect>
          </>
        ]
      }
    ];
  }

  render() {
    return (
      <>
        <FormGroup label="Listener" isRequired={true} fieldId="listener-port">
          <Table aria-label="Port Level MTLS" cells={listenerHeader} rows={this.portRows()}>
            <TableHeader />
            <TableBody />
          </Table>
        </FormGroup>
        <FormGroup fieldId="addRule">
          <Button
            variant={ButtonVariant.link}
            icon={<PlusCircleIcon />}
            onClick={this.onAddListener}
            isDisabled={!this.canAddListener()}
            className={addListenerStyle}
          >
            Add Listener to Listener List
          </Button>
          {!this.canAddListener() && <span className={warningStyle}>A Listener needs Hostname and Port sections defined</span>}
        </FormGroup>
      </>
    );
  }
}

export default ListenerBuilder;
