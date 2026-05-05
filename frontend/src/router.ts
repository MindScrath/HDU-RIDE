import { createRouter, createWebHistory } from 'vue-router'
import Login from './pages/Login.vue'
import ClassHome from './pages/ClassHome.vue'
import ClassMembers from './pages/ClassMembers.vue'
import LecturePage from './pages/LecturePage.vue'
import AssignmentPage from './pages/AssignmentPage.vue'
import AdminUsers from './pages/AdminUsers.vue'
import AdminCourseImport from './pages/AdminCourseImport.vue'
import { useSession } from './composables/useSession'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/classes' },
    { path: '/login', component: Login },
    { path: '/classes', component: ClassHome },
    { path: '/lectures/:lectureId?', component: LecturePage },
    { path: '/assignments/:assignmentId?', component: AssignmentPage },
    { path: '/classes/:classId/members', component: ClassMembers },
    { path: '/classes/:classId/lectures/:lectureId?', component: LecturePage },
    { path: '/classes/:classId/assignments/:assignmentId?', component: AssignmentPage },
    { path: '/admin/users', component: AdminUsers },
    { path: '/admin/courses', component: AdminCourseImport }
  ]
})

router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.state.initialized) {
    await session.fetchSession()
  }

  if (to.path !== '/login' && !session.signedIn.value) {
    return '/login'
  }
  if (to.path === '/login' && session.signedIn.value) {
    return '/classes'
  }
})

export default router
