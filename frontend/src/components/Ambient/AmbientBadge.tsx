import * as React from 'react';
import { Label, Tooltip, TooltipPosition } from '@patternfly/react-core';

type AmbientLabelProps = {
  style?: React.CSSProperties;
  tooltip: string;
};

export class AmbientBadge extends React.Component<AmbientLabelProps, {}> {
  render() {
    const tooltipContent = (
      <div style={{ textAlign: 'left' }}>
        <>
          <div>
            {this.props.tooltip}
            <br />
          </div>
        </>
      </div>
    );
    const iconComponent = (
      <span style={this.props.style}>
        <Label style={{ marginLeft: 5 }} color="blue" isCompact>
          Ambient
        </Label>
        {!this.props.tooltip && (
          <span style={{ marginLeft: '8px' }}>
            {this.props.tooltip}
            <Tooltip key={`tooltip_ambient_label`} position={TooltipPosition.top} content={tooltipContent}>
              <Label style={{ marginLeft: 5 }} color="blue" isCompact>
                Ambient
              </Label>
            </Tooltip>
          </span>
        )}
      </span>
    );
    return (
      <Tooltip key={`tooltip_ambient_label`} position={TooltipPosition.right} content={tooltipContent}>
        {iconComponent}
      </Tooltip>
    );
  }
}
