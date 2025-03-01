/// <reference path="../References.d.ts"/>
import * as React from 'react';
import * as MiscUtils from '../utils/MiscUtils';
import * as ImageTypes from '../types/ImageTypes';
import ImageDetailed from './ImageDetailed';
import OrganizationsStore from "../stores/OrganizationsStore";

interface Props {
	image: ImageTypes.ImageRo;
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
		paddingTop: '3px',
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
};

export default class Image extends React.Component<Props, {}> {
	render(): JSX.Element {
		let image = this.props.image;

		if (this.props.open) {
			return <div
				className="bp3-card bp3-row"
				style={css.cardOpen}
			>
				<ImageDetailed
					image={this.props.image}
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

		let orgClass = '';
		let orgIcon = '';
		let orgName = '';
		if (image.organization) {
			let org = OrganizationsStore.organization(image.organization);
			orgIcon = 'bp3-text-muted bp3-icon-people';
			orgName = org ? org.name : image.organization;
		} else {
			orgIcon = 'bp3-text-muted bp3-icon-globe';
			orgName = 'Public Image';
		}

		if (image.signed) {
			orgClass = 'bp3-text-intent-success';
			orgIcon = 'bp3-icon-endorsed';
			orgName = 'Signed Public Image';
		}

		let diskIcon = 'bp3-icon-box';
		switch (this.props.image.storage_class) {
			case 'aws_standard':
				diskIcon = 'bp3-icon-box';
				break;
			case 'aws_infrequent_access':
				diskIcon = 'bp3-icon-compressed';
				break;
			case 'aws_glacier':
				diskIcon = 'bp3-icon-snowflake';
				break;
			case 'oracle_standard':
				diskIcon = 'bp3-icon-box';
				break;
			case 'oracle_archive':
				diskIcon = 'bp3-icon-snowflake';
				break;
		}

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
						{image.name}
					</div>
				</div>
			</div>
			<div className={'bp3-cell ' + orgClass} style={css.item}>
				<span
					style={css.icon}
					className={'bp3-icon-standard ' + orgIcon}
				/>
				{orgName}
			</div>
			<div className="bp3-cell" style={css.item}>
				<span
					style={css.icon}
					hidden={!image.key}
					className={'bp3-icon-standard bp3-text-muted ' + diskIcon}
				/>
				{image.key}
			</div>
		</div>;
	}
}
