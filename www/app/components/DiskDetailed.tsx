/// <reference path="../References.d.ts"/>
import * as React from 'react';
import * as DiskTypes from '../types/DiskTypes';
import * as DiskActions from '../actions/DiskActions';
import * as OrganizationTypes from '../types/OrganizationTypes';
import PageInput from './PageInput';
import PageSelect from './PageSelect';
import PageSwitch from './PageSwitch';
import PageNumInput from './PageNumInput';
import PageInfo from './PageInfo';
import PageSelectButtonConfirm from './PageSelectButtonConfirm';
import Help from './Help';
import * as PageInfos from './PageInfo';
import PageSave from './PageSave';
import ConfirmButton from './ConfirmButton';
import NodesStore from '../stores/NodesStore';
import OrganizationsStore from '../stores/OrganizationsStore';
import * as InstanceActions from '../actions/InstanceActions';
import InstancesNodeStore from '../stores/InstancesNodeStore';
import * as InstanceTypes from '../types/InstanceTypes';
import * as Alert from '../Alert';
import PageTextArea from "./PageTextArea";

interface Props {
	organizations: OrganizationTypes.OrganizationsRo;
	disk: DiskTypes.DiskRo;
	selected: boolean;
	onSelect: (shift: boolean) => void;
	onClose: () => void;
}

interface State {
	disabled: boolean;
	changed: boolean;
	message: string;
	disk: DiskTypes.Disk;
	instances: InstanceTypes.InstancesRo;
	restoreImage: string;
	resizeDisk: boolean;
}

const css = {
	card: {
		position: 'relative',
		padding: '48px 10px 0 10px',
		width: '100%',
	} as React.CSSProperties,
	button: {
		height: '30px',
	} as React.CSSProperties,
	buttons: {
		cursor: 'pointer',
		position: 'absolute',
		top: 0,
		left: 0,
		right: 0,
		padding: '4px',
		height: '39px',
		backgroundColor: 'rgba(0, 0, 0, 0.13)',
	} as React.CSSProperties,
	item: {
		margin: '9px 5px 0 5px',
		height: '20px',
	} as React.CSSProperties,
	itemsLabel: {
		display: 'block',
	} as React.CSSProperties,
	itemsAdd: {
		margin: '8px 0 15px 0',
	} as React.CSSProperties,
	group: {
		flex: 1,
		minWidth: '280px',
		margin: '0 10px',
	} as React.CSSProperties,
	save: {
		paddingBottom: '10px',
	} as React.CSSProperties,
	label: {
		width: '100%',
		maxWidth: '280px',
	} as React.CSSProperties,
	status: {
		margin: '6px 0 0 1px',
	} as React.CSSProperties,
	icon: {
		marginRight: '3px',
	} as React.CSSProperties,
	inputGroup: {
		width: '100%',
	} as React.CSSProperties,
	protocol: {
		flex: '0 1 auto',
	} as React.CSSProperties,
	port: {
		flex: '1',
	} as React.CSSProperties,
	select: {
		margin: '7px 0px 0px 6px',
		paddingTop: '3px',
	} as React.CSSProperties,
	role: {
		margin: '9px 5px 0 5px',
		height: '20px',
	} as React.CSSProperties,
	rules: {
		marginBottom: '15px',
	} as React.CSSProperties,
};

export default class DiskDetailed extends React.Component<Props, State> {
	constructor(props: any, context: any) {
		super(props, context);
		this.state = {
			disabled: false,
			changed: false,
			message: '',
			disk: null,
			instances: null,
			restoreImage: null,
			resizeDisk: false,
		};
	}

	componentDidMount(): void {
		InstancesNodeStore.addChangeListener(this.onChange);
		InstanceActions.syncNode(this.props.disk.node);
	}

	componentWillUnmount(): void {
		InstancesNodeStore.removeChangeListener(this.onChange);
	}

	onChange = (): void => {
		this.setState({
			...this.state,
			instances: InstancesNodeStore.instances(this.props.disk.node),
		});
	}

	set(name: string, val: any): void {
		let disk: any;

		if (this.state.changed) {
			disk = {
				...this.state.disk,
			};
		} else {
			disk = {
				...this.props.disk,
			};
		}

		disk[name] = val;

		if (name === 'instance' && !Number(disk.index)) {
			disk['index'] = '0';
		}

		this.setState({
			...this.state,
			changed: true,
			disk: disk,
		});
	}

	setResizeDisk(val: boolean): void {
		let disk: any;

		if (this.state.changed) {
			disk = {
				...this.state.disk,
			};
		} else {
			disk = {
				...this.props.disk,
			};
		}

		if (val) {
			disk.new_size = disk.size;
		} else {
			disk.new_size = 0;
		}

		this.setState({
			...this.state,
			changed: true,
			resizeDisk: val,
			disk: disk,
		});

	}

	onSave = (): void => {
		this.setState({
			...this.state,
			disabled: true,
		});

		let disk = {
			...this.state.disk,
		};

		if (this.state.resizeDisk && disk.new_size > disk.size) {
			disk.state = 'expand';
		}

		DiskActions.commit(disk).then((): void => {
			this.setState({
				...this.state,
				message: 'Your changes have been saved',
				changed: false,
				disabled: false,
			});

			setTimeout((): void => {
				if (!this.state.changed) {
					this.setState({
						...this.state,
						disk: null,
						changed: false,
						resizeDisk: false,
					});
				}
			}, 1000);

			setTimeout((): void => {
				if (!this.state.changed) {
					this.setState({
						...this.state,
						message: '',
					});
				}
			}, 3000);
		}).catch((): void => {
			this.setState({
				...this.state,
				message: '',
				disabled: false,
			});
		});
	}

	onDelete = (): void => {
		this.setState({
			...this.state,
			disabled: true,
		});
		DiskActions.remove(this.props.disk.id).then((): void => {
			this.setState({
				...this.state,
				disabled: false,
			});
		}).catch((): void => {
			this.setState({
				...this.state,
				disabled: false,
			});
		});
	}

	onRestoreBackup = (): void => {
		let restoreImage: string;

		if (this.state.restoreImage) {
			restoreImage = this.state.restoreImage;
		} else if (this.props.disk.backups && this.props.disk.backups.length) {
			restoreImage = this.props.disk.backups[0].image;
		} else {
			return;
		}

		this.setState({
			...this.state,
			disabled: true,
		});

		let disk: DiskTypes.Disk;

		if (this.state.changed) {
			disk = {
				...this.state.disk,
			};
		} else {
			disk = {
				...this.props.disk,
			};
		}

		disk.state = 'restore';
		disk.restore_image = restoreImage;

		DiskActions.commit(disk).then((): void => {
			Alert.success('Disk restore started');

			this.setState({
				...this.state,
				disabled: false,
			});
		}).catch((): void => {
			this.setState({
				...this.state,
				disabled: false,
			});
		});
	}

	render(): JSX.Element {
		let disk: DiskTypes.Disk = this.state.disk ||
			this.props.disk;

		let node = NodesStore.node(this.props.disk.node);
		let org = OrganizationsStore.organization(this.props.disk.organization);

		let hasInstances = false;
		let instancesSelect: JSX.Element[] = [];
		if (this.state.instances === null) {
			instancesSelect = [<option key="null" value="">Loading</option>];
		} else {
			if (this.state.instances.length) {
				instancesSelect.push(
					<option key="null" value="">Detached Disk</option>);

				hasInstances = true;
				for (let instance of this.state.instances) {
					instancesSelect.push(
						<option
							key={instance.id}
							value={instance.id}
						>{instance.name}</option>,
					);
				}
			}

			if (!hasInstances) {
				instancesSelect = [<option key="null" value="">No Instances</option>];
			}
		}

		let backupsSelect: JSX.Element[] = [];
		for (let backup of (disk.backups || [])) {
			backupsSelect.push(
				<option key={backup.image} value={backup.image}>
					{backup.name}
				</option>,
			);
		}

		let hasBackups = false;
		if (!backupsSelect.length) {
			backupsSelect = [<option key="null" value="">No Backups</option>];
		} else {
			hasBackups = true
		}

		let statusText = 'Unknown';
		let statusClass = 'tab-close ';
		switch (this.props.disk.state) {
			case 'provision':
				statusText = 'Provisioning';
				statusClass += ' bp3-text-intent-primary';
				break;
			case 'available':
				if (this.props.disk.instance) {
					statusText = 'Connected';
				} else {
					statusText = 'Available';
				}
				statusClass += ' bp3-text-intent-success';
				break;
			case 'destroy':
				statusText = 'Destroying';
				statusClass += ' bp3-text-intent-danger';
				break;
			case 'snapshot':
				statusText = 'Snapshotting';
				statusClass += ' bp3-text-intent-primary';
				break;
			case 'backup':
				statusText = 'Backing Up';
				statusClass += ' bp3-text-intent-primary';
				break;
			case 'restore':
				statusText = 'Restoring';
				statusClass += ' bp3-text-intent-primary';
				break;
			case 'expand':
				statusText = 'Expanding';
				statusClass += ' bp3-text-intent-primary';
				break;
		}

		let fields: PageInfos.Field[] = [
			{
				label: 'ID',
				value: this.props.disk.id || 'Unknown',
			},
			{
				label: 'Image',
				value: this.props.disk.image || 'Blank Disk',
			},
			{
				label: 'Organization',
				value: org ? org.name : this.props.disk.organization,
			},
			{
				label: 'Node',
				value: (node ? node.name : this.props.disk.node) || '-',
			},
			{
				label: 'Size',
				value: this.props.disk.size + 'GB',
			},
		];

		let backingImage = this.props.disk.backing_image;
		if (backingImage) {
			backingImage = backingImage.replace('-', '\n');

			fields.splice(2, 0, {
				label: 'Backing Image',
				value: backingImage,
			});
		}

		return <td
			className="bp3-cell"
			colSpan={5}
			style={css.card}
		>
			<div className="layout horizontal wrap">
				<div style={css.group}>
					<div
						className="layout horizontal tab-close"
						style={css.buttons}
						onClick={(evt): void => {
							let target = evt.target as HTMLElement;

							if (target.className.indexOf('tab-close') !== -1) {
								this.props.onClose();
							}
						}}
					>
            <div>
              <label
                className="bp3-control bp3-checkbox tab-close"
                style={css.select}
              >
                <input
                  type="checkbox"
                  checked={this.props.selected}
									onChange={(evt): void => {
									}}
                  onClick={(evt): void => {
										this.props.onSelect(evt.shiftKey);
									}}
                />
                <span className="bp3-control-indicator"/>
              </label>
            </div>
						<div className={statusClass} style={css.status}>
							<span
								style={css.icon}
								className="bp3-icon-standard bp3-icon-pulse"
							/>
							{statusText}
						</div>
						<div className="flex tab-close"/>
						<ConfirmButton
							className="bp3-minimal bp3-intent-danger bp3-icon-trash"
							style={css.button}
							safe={true}
							progressClassName="bp3-intent-danger"
							dialogClassName="bp3-intent-danger bp3-icon-delete"
							dialogLabel="Delete Disk"
							confirmMsg="Permanently delete this disk"
							confirmInput={true}
							disabled={this.state.disabled}
							onConfirm={this.onDelete}
						/>
					</div>
					<PageInput
						label="Name"
						help="Name of disk."
						type="text"
						placeholder="Enter name"
						value={disk.name}
						onChange={(val): void => {
							this.set('name', val);
						}}
					/>
					<PageTextArea
						label="Comment"
						help="Disk comment."
						placeholder="Disk comment"
						rows={3}
						value={disk.comment}
						onChange={(val: string): void => {
							this.set('comment', val);
						}}
					/>
					<PageSelect
						disabled={this.state.disabled || !hasInstances}
						label="Instance"
						help="Instance to attach disk to."
						value={disk.instance}
						onChange={(val): void => {
							this.set('instance', val);
						}}
					>
						{instancesSelect}
					</PageSelect>
					<PageNumInput
						label="Index"
						help="Index to attach disk."
						hidden={!disk.instance}
						min={0}
						max={8}
						minorStepSize={1}
						stepSize={1}
						majorStepSize={1}
						disabled={this.state.disabled}
						selectAllOnFocus={true}
						value={Number(disk.index) || 0}
						onChange={(val: number): void => {
							this.set('index', String(val));
						}}
					/>
					<PageSwitch
						disabled={this.state.disabled}
						label="Delete protection"
						help="Block disk from being deleted."
						checked={disk.delete_protection}
						onToggle={(): void => {
							this.set('delete_protection', !disk.delete_protection);
						}}
					/>
				</div>
				<div style={css.group}>
					<PageInfo
						fields={fields}
					/>
					<PageSwitch
						disabled={this.state.disabled || disk.state != 'available'}
						label="Resize disk"
						help="Change size of disk. Instance will be stopped."
						checked={this.state.resizeDisk}
						onToggle={(): void => {
							this.setResizeDisk(!this.state.resizeDisk);
						}}
					/>
					<PageNumInput
						label="New Size"
						help="New disk size in gigabytes."
						hidden={!this.state.resizeDisk}
						min={disk.size}
						minorStepSize={1}
						stepSize={1}
						majorStepSize={1}
						disabled={this.state.disabled}
						selectAllOnFocus={true}
						value={disk.new_size}
						onChange={(val: number): void => {
							this.set('new_size', val);
						}}
					/>
					<PageSwitch
						disabled={this.state.disabled}
						label="Automatic backup"
						help="Automatically backup disk daily."
						checked={disk.backup}
						onToggle={(): void => {
							this.set('backup', !disk.backup);
						}}
					/>
					<label
						className="bp3-label"
						style={css.label}
					>
						Restore Backup
						<Help
							title="Restore Backup"
							content="Select a backup to restore and replace the existing disk with the backup image. Instance will be stopped."
						/>
					</label>
					<PageSelectButtonConfirm
						label="Restore"
						value={this.state.restoreImage}
						disabled={!hasBackups || this.state.disabled}
						confirmMsg="Confirm disk restore"
						buttonClass="bp3-intent-success bp3-icon-box"
						progressClassName="bp3-intent-success"
						onChange={(val: string): void => {
							this.setState({
								...this.state,
								restoreImage: val,
							});
						}}
						onSubmit={this.onRestoreBackup}
					>
						{backupsSelect}
					</PageSelectButtonConfirm>
				</div>
			</div>
			<PageSave
				style={css.save}
				hidden={!this.state.disk && !this.state.message}
				message={this.state.message}
				changed={this.state.changed}
				disabled={this.state.disabled}
				light={true}
				onCancel={(): void => {
					this.setState({
						...this.state,
						changed: false,
						disk: null,
					});
				}}
				onSave={this.onSave}
			/>
		</td>;
	}
}
