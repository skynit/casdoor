// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from "react";
import {Link} from "react-router-dom";
import {Button, Modal, Select, Table, message} from "antd";
import {DownloadOutlined, UploadOutlined} from "@ant-design/icons";
import moment from "moment";
import * as Setting from "./Setting";
import * as ActivationCodeBackend from "./backend/ActivationCodeBackend";
import i18next from "i18next";
import BaseListPage from "./BaseListPage";
import PopconfirmModal from "./common/modal/PopconfirmModal";

class ActivationCodeListPage extends BaseListPage {
  constructor(props) {
    super(props);
    this.state.pagination = {
      current: 1,
      pageSize: 10,
    };
    this.state.activated = "";
    this.state.stats = null;
  }

  componentDidMount() {
    this.fetchStats();
  }

  fetchStats() {
    ActivationCodeBackend.getActivationCodeStats("built-in")
      .then((res) => {
        if (res.status === "ok") {
          this.setState({stats: res.data});
        }
      });
  }

  getLabel(labelKey) {
    return Setting.getLabel(i18next.t(labelKey), i18next.t(`${labelKey} - Tooltip`));
  }

  deleteActivationCode(i) {
    ActivationCodeBackend.deleteActivationCode(this.state.data[i])
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully deleted"));
          this.fetch({
            pagination: {
              ...this.state.pagination,
              current: this.state.pagination.current > 1 && this.state.data.length === 1 ? this.state.pagination.current - 1 : this.state.pagination.current,
            },
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      });
  }

  resetActivationCode(i) {
    const record = this.state.data[i];
    const id = `${record.owner}/${record.name}`;
    ActivationCodeBackend.resetActivationCode(id)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully reset"));
          this.fetch({
            pagination: {
              ...this.state.pagination,
              current: this.state.pagination.current,
            },
          });
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to reset")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      });
  }

  parseRowData(row) {
    const values = row.split(",").map(v => v.trim());
    if (values.length < 4) {
      return null;
    }
    const statusNum = parseInt(values[2], 10);
    if (isNaN(statusNum) || (statusNum !== 0 && statusNum !== 1 && statusNum !== 2)) {
      return null;
    }
    return {
      name: values[0],
      owner: "built-in",
      createdTime: values[1] ? moment(values[1]).format() : moment().format(),
      status: statusNum,
      application: values[3] || "",
    };
  }

  handleImport(text) {
    const lines = text.split("\n").filter(line => line.trim() !== "");
    const codes = [];
    const errors = [];

    lines.forEach((line, index) => {
      const code = this.parseRowData(line);
      if (code) {
        codes.push(code);
      } else {
        errors.push(`Line ${index + 1}: invalid format`);
      }
    });

    if (errors.length > 0) {
      message.error(`${i18next.t("general:Failed to import")}: ${errors.slice(0, 3).join(", ")}`);
      return;
    }

    if (codes.length === 0) {
      message.error(i18next.t("general:No data to import"));
      return;
    }

    ActivationCodeBackend.addActivationCodes(codes)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("general:Successfully imported")} ${codes.length} ${i18next.t("beta:activation codes")}`);
          this.fetch();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to import")}: ${res.msg}`);
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
      });
  }

  handleExport() {
    const columns = [
      {title: "Name", dataIndex: "name", key: "name"},
      {title: "CreatedTime", dataIndex: "createdTime", key: "createdTime"},
      {title: "Status", dataIndex: "status", key: "status"},
      {title: "Application", dataIndex: "application", key: "application"},
    ];

    const data = this.state.data.map(item => ({
      name: item.name,
      createdTime: item.createdTime,
      status: item.status,
      application: item.application || "",
    }));

    const csvContent = [columns.map(c => c.title).join(",")]
      .concat(data.map(row => columns.map(c => row[c.dataIndex] || "").join(",")))
      .join("\n");

    const blob = new Blob([csvContent], {type: "text/csv;charset=utf-8;"});
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `activation-codes-${moment().format("YYYY-MM-DD")}.csv`;
    link.click();
  }

  showImportModal() {
    Modal.prompt({
      title: i18next.t("general:Import activation codes"),
      label: i18next.t("general:Paste CSV data"),
      placeholder: "name,createdTime,status,application\nCODE001,2024-01-01,0,app1",
      onChange: (value) => {
        this.promptValue = value;
      },
      onOk: () => {
        if (this.promptValue) {
          this.handleImport(this.promptValue);
        }
      },
    });
  }

  renderTable(codes) {
    const columns = [
      {
        title: i18next.t("general:Name"),
        dataIndex: "name",
        key: "name",
        width: 180,
        fixed: "left",
        sorter: true,
        ...this.getColumnSearchProps("name"),
        render: (text, record, index) => {
          return (
            <Link to={`/activation-codes/${record.owner}/${text}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("general:Organization"),
        dataIndex: "owner",
        key: "owner",
        width: 120,
        sorter: true,
        ...this.getColumnSearchProps("owner"),
        render: (text, record, index) => {
          return (
            <Link to={`/organizations/${text}`}>
              {text}
            </Link>
          );
        },
      },
      {
        title: i18next.t("general:Created time"),
        dataIndex: "createdTime",
        key: "createdTime",
        width: 160,
        sorter: true,
        render: (text, record, index) => {
          return Setting.getFormattedDate(text);
        },
      },
      {
        title: i18next.t("beta:Status"),
        dataIndex: "status",
        key: "status",
        width: 120,
        sorter: true,
        ...this.getColumnSearchProps("status"),
        render: (text, record, index) => {
          switch (text) {
          case 0:
            return Setting.getTag("success", i18next.t("beta:Available"));
          case 1:
            return Setting.getTag("processing", i18next.t("beta:Assigned"));
          case 2:
            return Setting.getTag("error", i18next.t("beta:Expired"));
          default:
            return text;
          }
        },
      },
      {
        title: i18next.t("general:Application"),
        dataIndex: "application",
        key: "application",
        width: 150,
        sorter: true,
        ...this.getColumnSearchProps("application"),
      },
      {
        title: i18next.t("beta:Assigned time"),
        dataIndex: "assignedAt",
        key: "assignedAt",
        width: 160,
        sorter: true,
        render: (text, record, index) => {
          return text ? Setting.getFormattedDate(text) : "-";
        },
      },
      {
        title: i18next.t("beta:Assigned to"),
        dataIndex: "assignedTo",
        key: "assignedTo",
        width: 150,
        sorter: true,
        ...this.getColumnSearchProps("assignedTo"),
        render: (text, record, index) => {
          return text || "-";
        },
      },
      {
        title: i18next.t("beta:Activated time"),
        dataIndex: "activatedAt",
        key: "activatedAt",
        width: 160,
        sorter: true,
        render: (text, record, index) => {
          return text ? Setting.getFormattedDate(text) : "-";
        },
      },
      {
        title: i18next.t("general:Action"),
        dataIndex: "operation",
        key: "operation",
        width: 180,
        fixed: "right",
        render: (text, record, index) => {
          return (
            <div>
              <PopconfirmModal
                onConfirm={() => this.resetActivationCode(index)}
                text={i18next.t("general:Reset")}
                title={i18next.t("general:Sure to reset")}
              />
              <PopconfirmModal
                onConfirm={() => this.deleteActivationCode(index)}
                text={i18next.t("general:Delete")}
                title={i18next.t("general:Sure to delete")}
              />
            </div>
          );
        },
      },
    ];
    const pagination = {
      total: this.state.pagination.total,
      current: this.state.pagination.current,
      pageSize: this.state.pagination.pageSize,
      showQuickJumper: true,
      showSizeChanger: true,
      pageSizeOptions: ["10", "20", "50", "100"],
      showTotal: (total, range) => `${range[0]}-${range[1]} of ${total}`,
    };

    return (
      <div>
        <Table
          scroll={{x: 1200}}
          columns={columns}
          dataSource={codes}
          rowKey={(record) => `${record.owner}/${record.name}`}
          pagination={pagination}
          loading={this.state.loading}
          onChange={this.handleTableChange}
        />
      </div>
    );
  }

  fetch = (params = {}) => {
    const sortField = params.sortField || "createdTime";
    const sortOrder = params.sortOrder || "descend";
    const pagination = params.pagination || this.state.pagination;

    this.setState({loading: true});
    const filter = {
      owner: "built-in",
      page: pagination.current,
      pageSize: pagination.pageSize,
      field: params.searchedColumn || "",
      value: params.searchText || "",
      sortField: sortField,
      sortOrder: sortOrder,
    };

    ActivationCodeBackend.getActivationCodes(filter.owner, filter.page, filter.pageSize, filter.field, filter.value, filter.sortField, filter.sortOrder, this.state.activated)
      .then((res) => {
        if (res.status === "ok") {
          this.setState({
            loading: false,
            data: res.data || [],
            pagination: {
              ...pagination,
              total: res.data2 || res.data?.length || 0,
            },
          });
        } else {
          Setting.showMessage("error", res.msg || i18next.t("general:Failed to load data"));
          this.setState({loading: false});
        }
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("general:Failed to connect to server")}: ${error}`);
        this.setState({loading: false});
      });
  };

  render() {
    if (this.state.loading && this.state.data === null) {
      return <div>{i18next.t("login:Loading")}</div>;
    }

    const statsBar = this.state.stats ? (
      <div style={{marginBottom: 16, display: "flex", gap: 24, fontSize: 14}}>
        <span>{i18next.t("beta:Total codes")}: <b>{this.state.stats.total}</b></span>
        <span>{i18next.t("beta:Assigned codes")}: <b>{this.state.stats.assigned}</b></span>
      </div>
    ) : null;

    const actionButtons = (
      <div>
        <Select
          value={this.state.activated}
          onChange={value => {
            this.setState({activated: value}, () => this.fetch({pagination: this.state.pagination}));
          }}
          placeholder={i18next.t("beta:Activation status")}
          style={{width: 120, marginRight: 8}}
          allowClear
        >
          <Select.Option value="true">{i18next.t("beta:Activated")}</Select.Option>
          <Select.Option value="false">{i18next.t("beta:Not activated")}</Select.Option>
        </Select>
        <Button type="primary" icon={<DownloadOutlined />} size="small" onClick={() => this.handleExport()} style={{marginRight: 8}}>
          {i18next.t("general:Export")}
        </Button>
        <Button icon={<UploadOutlined />} size="small" onClick={() => this.showImportModal()} style={{marginRight: 8}}>
          {i18next.t("general:Import")}
        </Button>
      </div>
    );

    return (
      <div>
        {statsBar}
        {actionButtons}
        {this.renderTable(this.state.data || [])}
      </div>
    );
  }
}

export default ActivationCodeListPage;
