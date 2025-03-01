/// <reference path="../References.d.ts"/>
import * as React from 'react';
import * as MiscUtils from '../utils/MiscUtils';
import * as BalancerTypes from '../types/BalancerTypes';
import * as OrganizationTypes from '../types/OrganizationTypes';
import * as CertificateTypes from '../types/CertificateTypes';
import * as DatacenterTypes from '../types/DatacenterTypes';
import OrganizationsStore from '../stores/OrganizationsStore';
import DatacentersStore from '../stores/DatacentersStore';
import BalancerDetailed from './BalancerDetailed';

interface Props {
	organizations: OrganizationTypes.OrganizationsRo;
	certificates: CertificateTypes.CertificatesRo;
	datacenters: DatacenterTypes.DatacentersRo;
	balancer: BalancerTypes.BalancerRo;
	selected: boolean;
	onSelect: (shift: boolean) => void;
	open: boolean;
	onOpen: () => void;
}

const css = {
	card: {
		display: 'table-row',
		width: '100%',
		padding: 0,
		boxShadow: 'none',
		cursor: 'pointer',
	} as React.CSSProperties,
	cardOpen: {
		display: 'table-row',
		width: '100%',
		padding: 0,
		boxShadow: 'none',
		position: 'relative',
	} as React.CSSProperties,
	select: {
		margin: '2px 0 0 0',
		paddingTop: '2px',
		minHeight: '18px',
	} as React.CSSProperties,
	name: {
		verticalAlign: 'top',
		display: 'table-cell',
		padding: '8px',
	} as React.CSSProperties,
	nameSpan: {
		margin: '1px 5px 0 0',
	} as React.CSSProperties,
	item: {
		verticalAlign: 'top',
		display: 'table-cell',
		padding: '9px',
		whiteSpace: 'nowrap',
	} as React.CSSProperties,
	icon: {
		marginRight: '3px',
	} as React.CSSProperties,
	bars: {
		verticalAlign: 'top',
		display: 'table-cell',
		padding: '8px',
		width: '30px',
	} as React.CSSProperties,
	bar: {
		height: '6px',
		marginBottom: '1px',
	} as React.CSSProperties,
	barLast: {
		height: '6px',
	} as React.CSSProperties,
	roles: {
		verticalAlign: 'top',
		display: 'table-cell',
		padding: '0 8px 8px 8px',
	} as React.CSSProperties,
	tag: {
		margin: '8px 5px 0 5px',
		height: '20px',
	} as React.CSSProperties,
};

export default class Balancer extends React.Component<Props, {}> {
	render(): JSX.Element {
		let balancer = this.props.balancer;

		if (this.props.open) {
			return <div
				className="bp3-card bp3-row"
				style={css.cardOpen}
			>
				<BalancerDetailed
					organizations={this.props.organizations}
					certificates={this.props.certificates}
					datacenters={this.props.datacenters}
					balancer={this.props.balancer}
					selected={this.props.selected}
					onSelect={this.props.onSelect}
					onClose={(): void => {
						this.props.onOpen();
					}}
				/>
			</div>;
		}

		let cardStyle = {
			...css.card,
		};

		let orgName = '';
		if (!MiscUtils.objectIdNil(balancer.organization)) {
			let org = OrganizationsStore.organization(balancer.organization);
			orgName = org ? org.name : balancer.organization;
		}

		let datacenter = DatacentersStore.datacenter(
			this.props.balancer.datacenter);
		let datacenterName = datacenter ? datacenter.name : null;

		return <div
			className="bp3-card bp3-row"
			style={cardStyle}
			onClick={(evt): void => {
				let target = evt.target as HTMLElement;

				if (target.className.indexOf('open-ignore') !== -1) {
					return;
				}

				this.props.onOpen();
			}}
		>
			<div className="bp3-cell" style={css.name}>
				<div className="layout horizontal">
					<label
						className="bp3-control bp3-checkbox open-ignore"
						style={css.select}
					>
						<input
							type="checkbox"
							className="open-ignore"
							checked={this.props.selected}
							onChange={(evt): void => {
							}}
							onClick={(evt): void => {
								this.props.onSelect(evt.shiftKey);
							}}
						/>
						<span className="bp3-control-indicator open-ignore"/>
					</label>
					<div style={css.nameSpan}>
						{balancer.name}
					</div>
				</div>
			</div>
			<div className="bp3-cell" style={css.item}>
				<span
					style={css.icon}
					hidden={!orgName}
					className={'bp3-icon-standard bp3-text-muted bp3-icon-people'}
				/>
				{orgName}
			</div>
			<div className="bp3-cell" style={css.item}>
				<span
					style={css.icon}
					hidden={!datacenterName}
					className="bp3-icon-standard bp3-text-muted bp3-icon-cloud"
				/>
				{datacenterName}
			</div>
		</div>;
	}
}
