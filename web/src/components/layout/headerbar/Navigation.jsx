/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button, Dropdown, Tooltip } from '@douyinfe/semi-ui';
import { MoreHorizontal } from 'lucide-react';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
}) => {
  const navigate = useNavigate();
  const visibleLinks = isMobile ? mainNavLinks.slice(0, 3) : mainNavLinks;
  const overflowLinks = isMobile ? mainNavLinks.slice(3) : [];

  const getTargetPath = (link) => {
    if (link.itemKey === 'console' && !userState.user) {
      return '/login';
    }
    if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
      return '/login';
    }
    return link.to;
  };

  const renderLink = (link, className) => {
    const linkContent = <span>{link.text}</span>;

    if (link.isExternal) {
      return (
        <a
          key={link.itemKey}
          href={link.externalLink}
          target='_blank'
          rel='noopener noreferrer'
          className={className}
        >
          {linkContent}
        </a>
      );
    }

    return (
      <Link key={link.itemKey} to={getTargetPath(link)} className={className}>
        {linkContent}
      </Link>
    );
  };

  const renderNavLinks = () => {
    const baseClasses = `flex-shrink-0 flex items-center font-semibold rounded-md transition-all duration-200 ease-in-out ${isMobile ? 'gap-0 text-sm' : 'gap-1'}`;
    const hoverClasses = 'hover:text-semi-color-primary';
    const spacingClasses = isMobile ? 'py-1' : 'p-2';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return visibleLinks.map((link) => renderLink(link, commonLinkClasses));
  };

  const openOverflowLink = (link) => {
    if (link.isExternal) {
      window.open(link.externalLink, '_blank', 'noopener,noreferrer');
      return;
    }
    navigate(getTargetPath(link));
  };

  const overflowLabel = overflowLinks.map((link) => link.text).join(' / ');

  return (
    <nav
      className={`flex min-w-0 flex-1 items-center whitespace-nowrap scrollbar-hide ${
        isMobile
          ? 'gap-1 mx-0 overflow-hidden'
          : 'gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto'
      }`}
    >
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        {renderNavLinks()}
        {overflowLinks.length > 0 && (
          <Tooltip content={overflowLabel}>
            <Dropdown
              position='bottomRight'
              render={
                <Dropdown.Menu>
                  {overflowLinks.map((link) => (
                    <Dropdown.Item
                      key={link.itemKey}
                      onClick={() => openOverflowLink(link)}
                    >
                      {link.text}
                    </Dropdown.Item>
                  ))}
                </Dropdown.Menu>
              }
            >
              <Button
                icon={<MoreHorizontal size={16} />}
                aria-label={overflowLabel}
                theme='borderless'
                type='tertiary'
                size='small'
                className='!p-1 !text-current !rounded-full'
              />
            </Dropdown>
          </Tooltip>
        )}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
