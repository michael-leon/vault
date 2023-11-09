/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { task, timeout } from 'ember-concurrency';
import { sanitizePath } from 'core/utils/sanitize-path';

export default Controller.extend({
  flashMessages: service(),
  vaultController: controller('vault'),
  clusterController: controller('vault.cluster'),
  namespaceService: service('namespace'),
  featureFlagService: service('featureFlag'),
  auth: service(),
  router: service(),
  permissions: service(),
  session: service(),
  queryParams: [{ authMethod: 'with', oidcProvider: 'o', mountPath: 'at' }],
  namespaceQueryParam: alias('clusterController.namespaceQueryParam'),
  wrappedToken: alias('vaultController.wrappedToken'),
  redirectTo: alias('vaultController.redirectTo'),
  managedNamespaceRoot: alias('featureFlagService.managedNamespaceRoot'),
  authMethod: '',
  oidcProvider: '',
  mountPath: '',

  get namespaceInput() {
    const namespaceQP = this.clusterController.namespaceQueryParam;
    if (this.managedNamespaceRoot) {
      // When managed, the user isn't allowed to edit the prefix `admin/` for their nested namespace
      const split = namespaceQP.split('/');
      if (split.length > 1) {
        split.shift();
        return `/${split.join('/')}`;
      }
      return '';
    }
    return namespaceQP;
  },

  fullNamespaceFromInput(value) {
    const strippedNs = sanitizePath(value);
    if (this.managedNamespaceRoot) {
      return `${this.managedNamespaceRoot}/${strippedNs}`;
    }
    return strippedNs;
  },

  updateNamespace: task(function* (value) {
    // debounce
    yield timeout(500);
    const ns = this.fullNamespaceFromInput(value);
    this.namespaceService.setNamespace(ns, true);
    this.set('namespaceQueryParam', ns);
  }).restartable(),

  authSuccess({ isRoot, namespace }) {
    let transition;
    if (this.redirectTo) {
      // here we don't need the namespace because it will be encoded in redirectTo
      transition = this.router.transitionTo(this.redirectTo);
      // reset the value on the controller because it's bound here
      this.set('redirectTo', '');
    } else {
      transition = this.router.transitionTo('vault.cluster', { queryParams: { namespace } });
    }
    transition.followRedirects().then(() => {
      if (isRoot) {
        this.auth.set('isRootToken', true);
        this.flashMessages.warning(
          'You have logged in with a root token. As a security precaution, this root token will not be stored by your browser and you will need to re-authenticate after the window is closed or refreshed.'
        );
      }
    });
  },

  actions: {
    onAuthResponse(authResponse, backend, data) {
      const { mfa_requirement } = authResponse;
      // if an mfa requirement exists further action is required
      if (mfa_requirement) {
        this.set('mfaAuthData', { mfa_requirement, backend, data });
      } else {
        this.authSuccess(authResponse);
      }
    },
    onMfaSuccess(authResponse) {
      this.authSuccess(authResponse);
    },
    onMfaErrorDismiss() {
      this.setProperties({
        mfaAuthData: null,
        mfaErrors: null,
      });
    },
    onParamUpdate(key, value) {
      if (key === 'namespace') {
        this.updateNamespace.perform(value);
      } else if (key === 'authType') {
        this.set('authMethod', value);
      } else if (key === 'mountPath') {
        this.set('mountPath', value);
      }
    },
    onSuccess() {
      this.permissions.getPaths.perform();
      if (this.session.data.authenticated.isRootToken) {
        this.flashMessages.warning(
          'You have logged in with a root token. As a security precaution, this root token will not be stored by your browser and you will need to re-authenticate after the window is closed or refreshed.'
        );
      }
    },
    cancelAuthentication() {
      this.set('cancelAuth', true);
      this.set('waitingForOktaNumberChallenge', false);
    },
  },
});
