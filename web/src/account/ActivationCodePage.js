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
import {Button, Card, Popconfirm, Spin, Typography} from "antd";
import {CheckCircleOutlined, CopyOutlined, ReloadOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as ActivationCodeBackend from "./ActivationCodeBackend";
import * as Setting from "../Setting";

const {Paragraph, Title, Text} = Typography;

class ActivationCodePage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      loading: true,
      status: "none",
      code: "",
      deviceId: "",
      activationTime: "",
    };
  }

  componentDidMount() {
    this.fetchStatus();
  }

  fetchStatus() {
    this.setState({loading: true});
    ActivationCodeBackend.getBetaStatus()
      .then((res) => {
        if (res.status === "ok") {
          if (res.data && res.data.status === "none") {
            this.setState({loading: false, status: "none"});
          } else if (res.data) {
            this.setState({
              loading: false,
              status: res.data.status || "none",
              code: res.data.code || "",
              deviceId: res.data.deviceId || "",
              activationTime: res.data.activationTime || "",
            });
          } else {
            this.setState({loading: false, status: "none"});
          }
        } else {
          Setting.showMessage("error", res.msg || "Error");
          this.setState({loading: false, status: "none"});
        }
      })
      .catch(() => {
        Setting.showMessage("error", "Network error");
        this.setState({loading: false, status: "none"});
      });
  }

  handleApply() {
    this.setState({loading: true});
    ActivationCodeBackend.applyBeta()
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Activation code applied successfully!");
          this.fetchStatus();
        } else {
          Setting.showMessage("error", res.msg || "Error");
          this.setState({loading: false});
        }
      })
      .catch(() => {
        Setting.showMessage("error", "Network error");
        this.setState({loading: false});
      });
  }

  handleReset() {
    this.setState({loading: true});
    ActivationCodeBackend.resetMyBeta()
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Activation reset successfully!");
          this.fetchStatus();
        } else {
          Setting.showMessage("error", res.msg || "Error");
          this.setState({loading: false});
        }
      })
      .catch(() => {
        Setting.showMessage("error", "Network error");
        this.setState({loading: false});
      });
  }

  renderNoApplication() {
    return (
      <Card>
        <Title level={3}>{i18next.t("beta:Activation Code")}</Title>
        <Paragraph>{i18next.t("beta:Apply for an activation code to activate your device.")}</Paragraph>
        <Button type="primary" size="large" onClick={() => this.handleApply()} loading={this.state.loading}>
          {i18next.t("beta:Apply for Activation Code")}
        </Button>
      </Card>
    );
  }

  renderPending() {
    return (
      <Card>
        <Title level={3}>{i18next.t("beta:Your Activation Code")}</Title>
        <Paragraph
          copyable={{
            text: this.state.code,
            icon: [
              <CopyOutlined key="copy" />,
              <CheckCircleOutlined key="copied" />,
            ],
          }}
        >
          <Text strong style={{fontSize: "20px", letterSpacing: "2px"}}>{this.state.code}</Text>
        </Paragraph>
      </Card>
    );
  }

  renderActivated() {
    return (
      <Card>
        <Title level={3}>{i18next.t("beta:Activation Code")}</Title>
        <Paragraph
          copyable={{
            text: this.state.code,
            icon: [
              <CopyOutlined key="copy" />,
              <CheckCircleOutlined key="copied" />,
            ],
          }}
        >
          <Text strong style={{fontSize: "20px", letterSpacing: "2px"}}>{this.state.code}</Text>
        </Paragraph>
        <Paragraph>
          <Text strong>Status: </Text>
          <Text type="success">Activated</Text>
        </Paragraph>
        {this.state.deviceId && (
          <Paragraph>
            <Text strong>Device ID: </Text>
            <Text code>{this.state.deviceId}</Text>
          </Paragraph>
        )}
        {this.state.activationTime && (
          <Paragraph>
            <Text strong>Activation Time: </Text>
            <Text>{this.state.activationTime}</Text>
          </Paragraph>
        )}
        <Popconfirm
          title="Are you sure to reset the activation? You will need to re-activate with a device."
          onConfirm={() => this.handleReset()}
          okText="Yes"
          cancelText="No"
        >
          <Button type="default" icon={<ReloadOutlined />} loading={this.state.loading}>
            Reset Activation
          </Button>
        </Popconfirm>
      </Card>
    );
  }

  render() {
    if (this.state.loading) {
      return <div style={{textAlign: "center", padding: "100px"}}><Spin size="large" /></div>;
    }

    if (this.state.status === "pending") {
      return this.renderPending();
    }
    if (this.state.status === "activated") {
      return this.renderActivated();
    }
    return this.renderNoApplication();
  }
}

export default ActivationCodePage;
